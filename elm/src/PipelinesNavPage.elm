port module PipelinesNavPage exposing (..)

import Html.App

import PipelinesNav

main : Program Never
main =
  Html.App.program
    { init = PipelinesNav.init
    , update = PipelinesNav.update
    , view = PipelinesNav.view
    , subscriptions = PipelinesNav.subscriptions
    }
