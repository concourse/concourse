port module Main exposing (main)

import Layout
import Navigation
import Routes
import SubPage

main : Program Never
main =
  Navigation.program
    (Navigation.makeParser Routes.parsePath)
    { init =
        Layout.init
          { init = SubPage.init
          , update = SubPage.update
          , view = SubPage.view
          , subscriptions = SubPage.subscriptions
          }
    , update = Layout.update
    , urlUpdate = Layout.urlUpdate
    , view = Layout.view
    , subscriptions = Layout.subscriptions
    }
