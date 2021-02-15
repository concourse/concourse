package main

var MigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "TODO",
	Long:  `TODO`,
	RunE:  InitializeMigrate,
}

func init() {
	atccmd.InitializeATCFlagsDEPRECATED(WebCommand, webFlagsDEPRECATED.RunCommand)
	atccmd.InitializePostgresFlags(MigrateCmd
	MigrateCmd.Flags().StringVar(&configFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
	EncryptionKey      flag.Cipher         `long:"encryption-key"     description:"A 16 or 32 length key used to encrypt sensitive information before storing it in the database."`
	CurrentDBVersion   bool                `long:"current-db-version" description:"Print the current database version and exit"`
	SupportedDBVersion bool                `long:"supported-db-version" description:"Print the max supported database version and exit"`
	MigrateDBToVersion int                 `long:"migrate-db-to-version" description:"Migrate to the specified database version and exit"`
}

func InitializeMigrate(cmd *cobra.Command, args []string) error {
	// Fetch all the flag values set
	//
	// XXX: When we stop supporting flags, we will need to replace this with a
	// new web object and fill in defaults manually with:
	// atccmd.SetDefaults(web.RunCommand)
	// tsacmd.SetDefaults(web.TSACommand)
	web := webFlagsDEPRECATED

	// IMPORTANT!! This can be removed after we completely deprecate flags
	fixupFlagDefaults(cmd, &web)

	// Fetch out env values
	env := envstruct.New("CONCOURSE", "yaml", envstruct.Parser{
		Delimiter:   ",",
		Unmarshaler: yaml.Unmarshal,
	})

	err := env.FetchEnv(web)
	if err != nil {
		return fmt.Errorf("fetch env: %s", err)
	}

	// Fetch out the values set from the config file and overwrite the flag
	// values
	if configFile != "" {
		file, err := os.Open(configFile)
		if err != nil {
			return fmt.Errorf("open file: %s", err)
		}

		decoder := yaml.NewDecoder(file)
		err = decoder.Decode(&web)
		if err != nil {
			return fmt.Errorf("decode config: %s", err)
		}
	}

	// Validate the values passed in by the user
	en := en.New()
	uni := ut.New(en, en)
	trans, _ := uni.GetTranslator("en")

	webValidator := atccmd.NewValidator(trans)

	err = webValidator.Struct(web)
	if err != nil {
		validationErrors := err.(validator.ValidationErrors)

		// TODO: FIX ERROR HANDLING
		var errOuts []string
		for _, err := range validationErrors {
			errOuts = append(errOuts, err.Translate(trans))
		}

		return fmt.Errorf(`TODO`)
	}

	err = web.Execute(cmd, args)
	if err != nil {
		return fmt.Errorf("failed to execute web: %s", err)
	}

	return nil
}
