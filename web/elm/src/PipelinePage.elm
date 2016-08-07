port module PipelinePage exposing (..)

import Html.App

import Pipeline

port fit : () -> Cmd msg

main : Program Pipeline.Flags
main =
  Html.App.programWithFlags
    { init = Pipeline.init fit
    , update = Pipeline.update
    , view = Pipeline.view
    , subscriptions = Pipeline.subscriptions
    }
