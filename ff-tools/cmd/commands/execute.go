package commands

func Execute() error {
	root, err := rootCmd()
	if err != nil {
		return err
	}

	return root.Execute()
}
