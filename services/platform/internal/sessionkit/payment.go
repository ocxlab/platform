package sessionkit

import "encoding/json"

const PayLaterHandler = "org.ocplab.internal.pay_later"
const PayLaterID = "ocp-pay-later"

// A selected internal instrument states when payment is due; it is not a PSP
// token or a customer assertion that money has already moved.
func ValidatePayLater(raw json.RawMessage) error {
	var request struct {
		Payment struct {
			Instruments []struct {
				ID        string `json:"id"`
				HandlerID string `json:"handler_id"`
				Type      string `json:"type"`
			} `json:"instruments"`
		} `json:"payment"`
	}
	if Decode(raw, &request) != nil || len(request.Payment.Instruments) != 1 {
		return ErrInput
	}
	i := request.Payment.Instruments[0]
	if i.ID != "pay-later" || i.HandlerID != PayLaterID || i.Type != "pay_later" {
		return ErrInput
	}
	return nil
}
