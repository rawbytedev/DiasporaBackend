use anchor_lang::prelude::*;
use anchor_spl::token::{self, CloseAccount, Mint, Token, TokenAccount, Transfer};
use std::str::FromStr;

declare_id!("5GHE14Zmpq5yNwpvHR2ZLaTcSckp6QogCRNm43M3Z9BT");

// This will be overwritten during testing via a build script,
// but for local tests you can manually set it to a test public key.
// We'll handle it automatically in the test script.
pub const TREASURY_AUTHORITY: &str = "11111111111111111111111111111111";

#[program]
pub mod diaspora_connect {
    use super::*;

    pub fn initiate_transfer(
        ctx: Context<InitiateTransfer>,
        amount: u64,
        nonce: u64,
    ) -> Result<()> {
        require!(amount > 0, ErrorCode::InvalidAmount);

        require_keys_neq!(
            ctx.accounts.sender.key(),
            ctx.accounts.recipient.key(),
            ErrorCode::SelfTransferNotAllowed
        );

        let clock = Clock::get()?;

        // 1% fee
        let fee_amount = amount.checked_div(100).ok_or(ErrorCode::MathOverflow)?;

        let escrow = &mut ctx.accounts.escrow_account;

        escrow.sender = ctx.accounts.sender.key();
        escrow.recipient = ctx.accounts.recipient.key();
        escrow.mint = ctx.accounts.mint.key();
        escrow.amount = amount;
        escrow.fee_amount = fee_amount;
        escrow.created_at = clock.unix_timestamp;
        escrow.expires_at = clock.unix_timestamp + 7 * 24 * 60 * 60;
        escrow.status = TransferStatus::Pending;
        escrow.nonce = nonce;
        escrow.bump = ctx.bumps.escrow_account;
        escrow.vault_bump = ctx.bumps.escrow_vault;

        let cpi_accounts = Transfer {
            from: ctx.accounts.sender_token_account.to_account_info(),
            to: ctx.accounts.escrow_vault.to_account_info(),
            authority: ctx.accounts.sender.to_account_info(),
        };

        let cpi_ctx = CpiContext::new(ctx.accounts.token_program.to_account_info(), cpi_accounts);

        token::transfer(cpi_ctx, amount)?;

        emit!(TransferInitiated {
            sender: escrow.sender,
            recipient: escrow.recipient,
            amount,
            nonce,
        });

        Ok(())
    }

    pub fn claim_transfer(ctx: Context<ClaimTransfer>, nonce: u64) -> Result<()> {
        let clock = Clock::get()?;

        let escrow_data = &ctx.accounts.escrow_account;

        require!(
            escrow_data.status == TransferStatus::Pending,
            ErrorCode::InvalidTransferState
        );

        require!(
            clock.unix_timestamp <= escrow_data.expires_at,
            ErrorCode::TransferExpired
        );

        require_keys_eq!(ctx.accounts.recipient.key(), escrow_data.recipient);

        let amount_to_recipient = escrow_data
            .amount
            .checked_sub(escrow_data.fee_amount)
            .ok_or(ErrorCode::MathOverflow)?;

        let fee_amount = escrow_data.fee_amount;

        let sender_key = escrow_data.sender;
        let recipient_key = escrow_data.recipient;
        let bump = escrow_data.bump;

        let nonce_bytes = nonce.to_le_bytes();
        let bump_seed = [bump];

        let seeds: &[&[u8]] = &[
            b"diaspora-escrow",
            sender_key.as_ref(),
            recipient_key.as_ref(),
            &nonce_bytes,
            &bump_seed,
        ];

        let signer_seeds: &[&[&[u8]]] = &[seeds];

        // Transfer to recipient
        let cpi_accounts_recipient = Transfer {
            from: ctx.accounts.escrow_vault.to_account_info(),
            to: ctx.accounts.recipient_token_account.to_account_info(),
            authority: ctx.accounts.escrow_account.to_account_info(),
        };

        let cpi_ctx_recipient = CpiContext::new_with_signer(
            ctx.accounts.token_program.to_account_info(),
            cpi_accounts_recipient,
            signer_seeds,
        );

        token::transfer(cpi_ctx_recipient, amount_to_recipient)?;

        // Transfer fee
        if fee_amount > 0 {
            let cpi_accounts_fee = Transfer {
                from: ctx.accounts.escrow_vault.to_account_info(),
                to: ctx.accounts.fee_treasury.to_account_info(),
                authority: ctx.accounts.escrow_account.to_account_info(),
            };

            let cpi_ctx_fee = CpiContext::new_with_signer(
                ctx.accounts.token_program.to_account_info(),
                cpi_accounts_fee,
                signer_seeds,
            );

            token::transfer(cpi_ctx_fee, fee_amount)?;
        }

        // Close vault
        let close_accounts = CloseAccount {
            account: ctx.accounts.escrow_vault.to_account_info(),
            destination: ctx.accounts.sender.to_account_info(),
            authority: ctx.accounts.escrow_account.to_account_info(),
        };

        let close_ctx = CpiContext::new_with_signer(
            ctx.accounts.token_program.to_account_info(),
            close_accounts,
            signer_seeds,
        );

        token::close_account(close_ctx)?;

        ctx.accounts.escrow_account.status = TransferStatus::Claimed;

        emit!(TransferClaimed {
            sender: sender_key,
            recipient: recipient_key,
            amount: amount_to_recipient,
            nonce,
        });

        Ok(())
    }

    pub fn refund_transfer(ctx: Context<RefundTransfer>, nonce: u64) -> Result<()> {
        let clock = Clock::get()?;

        let escrow_data = &ctx.accounts.escrow_account;

        require!(
            escrow_data.status == TransferStatus::Pending,
            ErrorCode::InvalidTransferState
        );

        require!(
            clock.unix_timestamp > escrow_data.expires_at,
            ErrorCode::RefundNotAvailable
        );

        require_keys_eq!(ctx.accounts.sender.key(), escrow_data.sender);

        let amount = escrow_data.amount;

        let sender_key = escrow_data.sender;
        let recipient_key = escrow_data.recipient;
        let bump = escrow_data.bump;

        let nonce_bytes = nonce.to_le_bytes();
        let bump_seed = [bump];

        let seeds: &[&[u8]] = &[
            b"diaspora-escrow",
            sender_key.as_ref(),
            recipient_key.as_ref(),
            &nonce_bytes,
            &bump_seed,
        ];

        let signer_seeds: &[&[&[u8]]] = &[seeds];

        let cpi_accounts = Transfer {
            from: ctx.accounts.escrow_vault.to_account_info(),
            to: ctx.accounts.sender_token_account.to_account_info(),
            authority: ctx.accounts.escrow_account.to_account_info(),
        };

        let cpi_ctx = CpiContext::new_with_signer(
            ctx.accounts.token_program.to_account_info(),
            cpi_accounts,
            signer_seeds,
        );

        token::transfer(cpi_ctx, amount)?;

        // Close vault
        let close_accounts = CloseAccount {
            account: ctx.accounts.escrow_vault.to_account_info(),
            destination: ctx.accounts.sender.to_account_info(),
            authority: ctx.accounts.escrow_account.to_account_info(),
        };

        let close_ctx = CpiContext::new_with_signer(
            ctx.accounts.token_program.to_account_info(),
            close_accounts,
            signer_seeds,
        );

        token::close_account(close_ctx)?;

        ctx.accounts.escrow_account.status = TransferStatus::Refunded;

        emit!(TransferRefunded {
            sender: sender_key,
            recipient: recipient_key,
            amount,
            nonce,
        });

        Ok(())
    }
}

#[derive(Accounts)]
#[instruction(nonce: u64)]
pub struct InitiateTransfer<'info> {
    #[account(mut)]
    pub sender: Signer<'info>,

    #[account(
        mut,
        constraint = sender_token_account.owner == sender.key(),
        constraint = sender_token_account.mint == mint.key(),
        constraint = sender_token_account.delegate.is_none()
    )]
    pub sender_token_account: Account<'info, TokenAccount>,

    /// CHECK:
    pub recipient: UncheckedAccount<'info>,

    #[account(
        mut,
        constraint = fee_treasury.mint == mint.key(),
        constraint = fee_treasury.owner
            == Pubkey::from_str(TREASURY_AUTHORITY).unwrap()
    )]
    pub fee_treasury: Account<'info, TokenAccount>,

    pub mint: Account<'info, Mint>,

    #[account(
        init,
        payer = sender,
        space = 8 + EscrowAccount::LEN,
        seeds = [
            b"diaspora-escrow",
            sender.key.as_ref(),
            recipient.key.as_ref(),
            &nonce.to_le_bytes()
        ],
        bump,
    )]
    pub escrow_account: Account<'info, EscrowAccount>,

    #[account(
        init,
        payer = sender,
        token::mint = mint,
        token::authority = escrow_account,
        seeds = [
            b"diaspora-vault",
            sender.key.as_ref(),
            recipient.key.as_ref(),
            &nonce.to_le_bytes()
        ],
        bump,
    )]
    pub escrow_vault: Account<'info, TokenAccount>,

    pub token_program: Program<'info, Token>,
    pub system_program: Program<'info, System>,
    pub rent: Sysvar<'info, Rent>,
}

#[derive(Accounts)]
#[instruction(nonce: u64)]
pub struct ClaimTransfer<'info> {
    #[account(mut)]
    pub recipient: Signer<'info>,

    #[account(
        mut,
        constraint = recipient_token_account.owner == recipient.key(),
        constraint = recipient_token_account.mint == mint.key(),
        constraint = recipient_token_account.delegate.is_none()
    )]
    pub recipient_token_account: Account<'info, TokenAccount>,

    /// CHECK:
    pub sender: UncheckedAccount<'info>,

    #[account(
        mut,
        has_one = sender,
        has_one = recipient,
        has_one = mint,
        seeds = [
            b"diaspora-escrow",
            sender.key.as_ref(),
            recipient.key.as_ref(),
            &nonce.to_le_bytes()
        ],
        bump = escrow_account.bump,
    )]
    pub escrow_account: Account<'info, EscrowAccount>,

    #[account(
        mut,
        constraint = escrow_vault.mint == mint.key(),
        constraint = escrow_vault.delegate.is_none(),
        seeds = [
            b"diaspora-vault",
            sender.key.as_ref(),
            recipient.key.as_ref(),
            &nonce.to_le_bytes()
        ],
        bump = escrow_account.vault_bump,
    )]
    pub escrow_vault: Account<'info, TokenAccount>,

    #[account(
        mut,
        constraint = fee_treasury.mint == mint.key(),
        constraint = fee_treasury.owner
            == Pubkey::from_str(TREASURY_AUTHORITY).unwrap()
    )]
    pub fee_treasury: Account<'info, TokenAccount>,

    pub mint: Account<'info, Mint>,
    pub token_program: Program<'info, Token>,
}

#[derive(Accounts)]
#[instruction(nonce: u64)]
pub struct RefundTransfer<'info> {
    #[account(mut)]
    pub sender: Signer<'info>,

    #[account(
        mut,
        constraint = sender_token_account.owner == sender.key(),
        constraint = sender_token_account.mint == mint.key(),
        constraint = sender_token_account.delegate.is_none()
    )]
    pub sender_token_account: Account<'info, TokenAccount>,

    /// CHECK:
    pub recipient: UncheckedAccount<'info>,

    #[account(
        mut,
        has_one = sender,
        has_one = recipient,
        has_one = mint,
        seeds = [
            b"diaspora-escrow",
            sender.key.as_ref(),
            recipient.key.as_ref(),
            &nonce.to_le_bytes()
        ],
        bump = escrow_account.bump,
    )]
    pub escrow_account: Account<'info, EscrowAccount>,

    #[account(
        mut,
        constraint = escrow_vault.mint == mint.key(),
        constraint = escrow_vault.delegate.is_none(),
        seeds = [
            b"diaspora-vault",
            sender.key.as_ref(),
            recipient.key.as_ref(),
            &nonce.to_le_bytes()
        ],
        bump = escrow_account.vault_bump,
    )]
    pub escrow_vault: Account<'info, TokenAccount>,

    pub mint: Account<'info, Mint>,
    pub token_program: Program<'info, Token>,
}

#[account]
pub struct EscrowAccount {
    pub sender: Pubkey,
    pub recipient: Pubkey,
    pub mint: Pubkey,
    pub amount: u64,
    pub fee_amount: u64,
    pub created_at: i64,
    pub expires_at: i64,
    pub status: TransferStatus,
    pub nonce: u64,
    pub bump: u8,
    pub vault_bump: u8,
}

impl EscrowAccount {
    pub const LEN: usize = 32 + // sender
        32 + // recipient
        32 + // mint
        8 +  // amount
        8 +  // fee_amount
        8 +  // created_at
        8 +  // expires_at
        1 +  // status
        8 +  // nonce
        1 +  // bump
        1; // vault_bump
}

#[derive(AnchorSerialize, AnchorDeserialize, Clone, PartialEq, Eq)]
pub enum TransferStatus {
    Pending,
    Claimed,
    Refunded,
}

#[event]
pub struct TransferInitiated {
    pub sender: Pubkey,
    pub recipient: Pubkey,
    pub amount: u64,
    pub nonce: u64,
}

#[event]
pub struct TransferClaimed {
    pub sender: Pubkey,
    pub recipient: Pubkey,
    pub amount: u64,
    pub nonce: u64,
}

#[event]
pub struct TransferRefunded {
    pub sender: Pubkey,
    pub recipient: Pubkey,
    pub amount: u64,
    pub nonce: u64,
}

#[error_code]
pub enum ErrorCode {
    #[msg("Invalid transfer amount")]
    InvalidAmount,

    #[msg("Transfer not in a pending state")]
    InvalidTransferState,

    #[msg("Transfer has expired")]
    TransferExpired,

    #[msg("Refund is not available yet")]
    RefundNotAvailable,

    #[msg("Math overflow")]
    MathOverflow,

    #[msg("Sender and recipient cannot be the same")]
    SelfTransferNotAllowed,
}
 