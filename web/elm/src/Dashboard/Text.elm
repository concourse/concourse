module Dashboard.Text exposing
    ( asciiArt
    , cliInstructions
    , setPipelineInstructions
    , welcome
    )


asciiArt : String
asciiArt =
    String.join "\n"
        [ "                          `:::                                             "
        , "                         `:::::                                            "
        , "                         :::::::                                           "
        , "                         ::::::::`                                         "
        , "                          ::::::::,           :                            "
        , "                           :::::::::      ::: ::                           "
        , "                            :::::::::    :::::` ,                          "
        , "                             :::::::::  :::::::                            "
        , "                              :::::::::::::::::`                           "
        , "                               ::::::::::::::::                            "
        , "                                ::::::::::::::.                            "
        , "                           `:`   ::::::, `:::.                             "
        , "                          `:.     ::::,  :::.                              "
        , "                      :: `:.      :::,  ::::                               "
        , "                     :: `:.      ::::  ::::::                              "
        , "                    :: `:.      ,:::::::::::::                             "
        , "                   ::  :.      .:::::::::::::::                            "
        , "                  ,:           ::::::::::::::::.                           "
        , "                              ::::::::. ::::::::`                          "
        , "                             ::::::::`   ::::::::                `         "
        , "                            ::::::::      ::::::::               ::`       "
        , "                           ,:::::::        ::::::::              ,::,  . ` "
        , "                         :::::::::          ::::::::              ,:::,::  "
        , "                        ::::::::.            :::::::.              ,:::::  "
        , "                       ::::::::`              :::::::             ` :: :   "
        , "                      .:::::::                 :::::          `  : .:,::,  "
        , "                       .::::::            :.    :::          `     :::,::. "
        , "                        .:::::      .:   :.      .            .   :::  ,::`"
        , "                         .:::      .:   :.                   ,  ,::,    ,::"
        , "                          .,      .:   :.                   ,   :::      ,:"
        , "                                 .:   :.                         :  .      "
        , "                                `:   :.                            ` :     "
        , "                                                                    :      "
        , "    .                                                              `       "
        , "    ::                                                                     "
        , "     ::,:                                                                  "
        , "      : :                                                                  "
        , "     `:::                                                                  "
        , "     :, ::                                                                 "
        , "   .:.   :,                                                                "
        , "    :                                                                      "
        , "        `                                                                  "
        , "       .                                                                   "
        ]


welcome : String
welcome =
    "welcome to concourse!"


cliInstructions : String
cliInstructions =
    "first, download the CLI tools:"


setPipelineInstructions : String
setPipelineInstructions =
    "then, use `fly set-pipeline` to set up your new pipeline"
