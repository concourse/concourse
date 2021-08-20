package atc

import "errors"

type Webhook struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Token string `json:"token"`
}

func (w Webhook) Validate() error {
	if w.Name == "" {
		return errors.New("missing name")
	}
	warning, err := ValidateIdentifier(w.Name, "webhook", "name")
	if err != nil {
		return err
	}
	if warning != nil {
		return errors.New(warning.Message)
	}
	if w.Type == "" {
		return errors.New("missing type")
	}
	if w.Token == "" {
		return errors.New("missing token")
	}
	return nil
}
