port module Main exposing (main)

import Layout
import Navigation


main : Program Layout.Flags Layout.Model Layout.Msg
main =
    Navigation.programWithFlags Layout.locationMsg
        { init = Layout.init
        , update = Layout.update
        , view = Layout.view
        , subscriptions = Layout.subscriptions
        }
