module ResourcePage exposing (main)

import Html.App

import Resource

main : Program Resource.Flags
main =
  Html.App.programWithFlags
    { init = Resource.init
    , update = Resource.update
    , view = Resource.view
    , subscriptions = Resource.autoupdateTimer
  }
