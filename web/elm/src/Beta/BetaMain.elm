port module BetaMain exposing (main)

import BetaLayout
import Navigation


main : Program BetaLayout.Flags BetaLayout.Model BetaLayout.Msg
main =
    Navigation.programWithFlags BetaLayout.locationMsg
        { init = BetaLayout.init
        , update = BetaLayout.update
        , view = BetaLayout.view
        , subscriptions = BetaLayout.subscriptions
        }
