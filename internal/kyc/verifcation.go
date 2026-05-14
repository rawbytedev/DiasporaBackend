package kyc

// stripe services will be used

type KYClient struct {
	APiurl string
}

func NewKYC(api string) *KYClient {
	return &KYClient{
		APiurl: api,
	}
}

// verify that a customer has completed verification
func (k *KYClient) Verify() {

}

// Sends over details regarding a specific customer for kyc
func (k *KYClient) AttachInfo() {

}
