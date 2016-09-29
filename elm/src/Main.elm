port module Main exposing (main)

import Layout
import Navigation
import Routes

main : Program Layout.Flags
main =
  Navigation.programWithFlags
    (Navigation.makeParser Routes.parsePath)
    { init = Layout.init
    , update = Layout.update
    , urlUpdate = Layout.urlUpdate
    , view = Layout.view
    , subscriptions = Layout.subscriptions
    }
