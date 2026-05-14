use anchor_lang::prelude::*;
use anchor_spl::token::{self, Mint, Token, TokenAccount, Transfer};

declare_id!("Diaspora1111111111111111111111111111111111111");

#[program]
pub mod diaspora_connect {
    use super::*;

    pub fn initiate_transfer(
        ctx: Context<InitiateTransfer>,
        amount: u64,
        nonce: u64,
    ) -> Result<()> {
        require!(amount > 0, ErrorCode::InvalidAmount);

        let escrow = &mut ctx.accounts.escrow_account;
        let clock = Clock::get()?;
        let fee_amount = amount.checked_div(100).unwrap_or(0);

        escrow.sender = *ctx.accounts.sender.key;
        escrow.recipient = ctx.accounts.recipient.key();
        escrow.mint = ctx.accounts.mint.key();
        escrow.amount = amount;
        escrow.fee_amount = fee_amount;
        escrow.created_at = clock.unix_timestamp;
        escrow.expires_at = clock.unix_timestamp + 7 * 24 * 60 * 60;
        escrow.status = TransferStatus::Pending;
        escrow.nonce = nonce;
        escrow.bump = *ctx.bumps.get("escrow_account").unwrap();
        escrow.vault_bump = *ctx.bumps.get("escrow_vault").unwrap();

        let cpi_accounts = Transfer {
            from: ctx.accounts.sender_token_account.to_account_info(),
            to: ctx.accounts.escrow_vault.to_account_info(),
            authority: ctx.accounts.sender.to_account_info(),
        };
        let cpi_ctx = CpiContext::new(ctx.accounts.token_program.to_account_info(), cpi_accounts);
        token::transfer(cpi_ctx, amount)
    }

    pub fn claim_transfer(ctx: Context<ClaimTransfer>, nonce: u64) -> Result<()> {
        let escrow = &mut ctx.accounts.escrow_account;
        let clock = Clock::get()?;

        require!(escrow.status == TransferStatus::Pending, ErrorCode::InvalidTransferState);
        require!(clock.unix_timestamp <= escrow.expires_at, ErrorCode::TransferExpired);
        require_keys_eq!(ctx.accounts.recipient.key(), escrow.recipient);

        let amount_to_recipient = escrow.amount.checked_sub(escrow.fee_amount).unwrap();

        let cpi_accounts_recipient = Transfer {
            from: ctx.accounts.escrow_vault.to_account_info(),
            to: ctx.accounts.recipient_token_account.to_account_info(),
            authority: ctx.accounts.escrow_account.to_account_info(),
        };
        let signer_seeds = &[&[
            b"diaspora-escrow",
            ctx.accounts.sender.key.as_ref(),
            ctx.accounts.recipient.key.as_ref(),
            &nonce.to_le_bytes(),
            &[escrow.bump],
        ]][..];
        let cpi_ctx_recipient = CpiContext::new_with_signer(
            ctx.accounts.token_program.to_account_info(),
            cpi_accounts_recipient,
            signer_seeds,
        );
        token::transfer(cpi_ctx_recipient, amount_to_recipient)?;

        if escrow.fee_amount > 0 {
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
            token::transfer(cpi_ctx_fee, escrow.fee_amount)?;
        }

        escrow.status = TransferStatus::Claimed;
        Ok(())
    }

    pub fn refund_transfer(ctx: Context<RefundTransfer>, nonce: u64) -> Result<()> {
        let escrow = &mut ctx.accounts.escrow_account;
        let clock = Clock::get()?;

        require!(escrow.status == TransferStatus::Pending, ErrorCode::InvalidTransferState);
        require!(clock.unix_timestamp > escrow.expires_at, ErrorCode::RefundNotAvailable);
        require_keys_eq!(ctx.accounts.sender.key(), escrow.sender);

        let cpi_accounts = Transfer {
            from: ctx.accounts.escrow_vault.to_account_info(),
            to: ctx.accounts.sender_token_account.to_account_info(),
            authority: ctx.accounts.escrow_account.to_account_info(),
        };
        let signer_seeds = &[&[
            b"diaspora-escrow",
            ctx.accounts.sender.key.as_ref(),
            ctx.accounts.recipient.key.as_ref(),
            &nonce.to_le_bytes(),
            &[escrow.bump],
        ]][..];
        let cpi_ctx = CpiContext::new_with_signer(
            ctx.accounts.token_program.to_account_info(),
            cpi_accounts,
            signer_seeds,
        );
        token::transfer(cpi_ctx, escrow.amount)?;

        escrow.status = TransferStatus::Refunded;
        Ok(())
    }
}

#[derive(Accounts)]
pub struct InitiateTransfer<'info> {
    #[account(mut)]
    pub sender: Signer<'info>,

    #[account(mut, constraint = sender_token_account.owner == sender.key())]
    pub sender_token_account: Account<'info, TokenAccount>,

    /// CHECK: recipient is validated by the escrow PDA seed
    pub recipient: UncheckedAccount<'info>,

    #[account(mut)]
    pub fee_treasury: Account<'info, TokenAccount>,

    pub mint: Account<'info, Mint>,

    #[account(
        init,
        payer = sender,
        space = 8 + EscrowAccount::LEN,
        seeds = [b"diaspora-escrow", sender.key.as_ref(), recipient.key.as_ref(), &nonce.to_le_bytes()],
        bump,
    )]
    pub escrow_account: Account<'info, EscrowAccount>,

    #[account(
        init,
        payer = sender,
        token::mint = mint,
        token::authority = escrow_account,
        seeds = [b"diaspora-vault", sender.key.as_ref(), recipient.key.as_ref(), &nonce.to_le_bytes()],
        bump,
    )]
    pub escrow_vault: Account<'info, TokenAccount>,

    pub token_program: Program<'info, Token>,
    pub system_program: Program<'info, System>,
    pub rent: Sysvar<'info, Rent>,
}

#[derive(Accounts)]
pub struct ClaimTransfer<'info> {
    #[account(mut)]
    pub recipient: Signer<'info>,

    #[account(mut, constraint = recipient_token_account.owner == recipient.key())]
    pub recipient_token_account: Account<'info, TokenAccount>,

    /// CHECK: sender is validated by escrow account content and PDA seeds
    pub sender: UncheckedAccount<'info>,

    #[account(
        mut,
        seeds = [b"diaspora-escrow", sender.key.as_ref(), recipient.key.as_ref(), &nonce.to_le_bytes()],
        bump = escrow_account.bump,
        has_one = recipient,
        has_one = sender,
        has_one = mint,
    )]
    pub escrow_account: Account<'info, EscrowAccount>,

    #[account(mut,
        seeds = [b"diaspora-vault", sender.key.as_ref(), recipient.key.as_ref(), &nonce.to_le_bytes()],
        bump = escrow_account.vault_bump,
    )]
    pub escrow_vault: Account<'info, TokenAccount>,

    #[account(mut, constraint = fee_treasury.mint == mint.key())]
    pub fee_treasury: Account<'info, TokenAccount>,

    pub mint: Account<'info, Mint>,
    pub token_program: Program<'info, Token>,
}

#[derive(Accounts)]
pub struct RefundTransfer<'info> {
    #[account(mut)]
    pub sender: Signer<'info>,

    #[account(mut, constraint = sender_token_account.owner == sender.key())]
    pub sender_token_account: Account<'info, TokenAccount>,

    /// CHECK: recipient is validated by escrow account content and PDA seeds
    pub recipient: UncheckedAccount<'info>,

    #[account(
        mut,
        seeds = [b"diaspora-escrow", sender.key.as_ref(), recipient.key.as_ref(), &nonce.to_le_bytes()],
        bump = escrow_account.bump,
        has_one = sender,
        has_one = recipient,
        has_one = mint,
    )]
    pub escrow_account: Account<'info, EscrowAccount>,

    #[account(mut,
        seeds = [b"diaspora-vault", sender.key.as_ref(), recipient.key.as_ref(), &nonce.to_le_bytes()],
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
    pub const LEN: usize = 32 + 32 + 32 + 8 + 8 + 8 + 8 + 1 + 8 + 1 + 1;
}

#[derive(AnchorSerialize, AnchorDeserialize, Clone, PartialEq, Eq)]
pub enum TransferStatus {
    Pending,
    Claimed,
    Refunded,
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
}
