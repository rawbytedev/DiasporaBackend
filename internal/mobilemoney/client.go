package mobilemoney

type Client struct {
	apiURL    string
	apiKey    string
	apiSecret string
}

func NewClient(apiURL, apiKey, apiSecret string) *Client {
	return &Client{
		apiURL:    apiURL,
		apiKey:    apiKey,
		apiSecret: apiSecret,
	}
}

func (c *Client) SendMoney(phone string, amount float64, provider string) (string, error) {
	// implémentation de l'appel à l'API du fournisseur mobile money (MTN, Moov, etc.)
	// retourne un ID de transaction ou une erreur
	return "mock_tx_id", nil
}

func (c *Client) CheckTransactionStatus(txID string) (string, error) {
	// implémentation de l'appel à l'API pour vérifier le statut d'une transaction
	// retourne "pending", "success", "failed", etc. ou une erreur
	return "success", nil
}
