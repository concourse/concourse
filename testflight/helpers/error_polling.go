package helpers

func ErrorPolling(url string) func() error {
	client := httpClient()

	return func() error {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
		}

		return err
	}
}
