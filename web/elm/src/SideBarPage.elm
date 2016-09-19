port module PipelinesNavPage exposing (..)

import Html.App

import SideBar

main : Program Never
main =
  Html.App.program
    { init = SideBar.init
    , update = SideBar.update
    , view = SideBar.view
    , subscriptions = SideBar.subscriptions
    }
