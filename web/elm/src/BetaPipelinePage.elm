port module BetaPipelinePage exposing (main)

import Html.App

import BetaPipeline

port setGroups : (List String -> msg) -> Sub msg

main : Program BetaPipeline.Flags
main =
  Html.App.programWithFlags
    { init = BetaPipeline.init { setGroups = setGroups }
    , update = BetaPipeline.update
    , view = BetaPipeline.view
    , subscriptions = BetaPipeline.subscriptions
    }
