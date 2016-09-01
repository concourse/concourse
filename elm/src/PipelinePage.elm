port module PipelinePage exposing (main)

import Html.App

import Pipeline

port setGroups : (List String -> msg) -> Sub msg

main : Program Pipeline.Flags
main =
  Html.App.programWithFlags
    { init = Pipeline.init { setGroups = setGroups }
    , update = Pipeline.update
    , view = Pipeline.view
    , subscriptions = Pipeline.subscriptions
    }
