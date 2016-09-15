port module PipelinePage exposing (main)

import Html.App
import Json.Encode

import Pipeline

port renderPipeline : (Json.Encode.Value, Json.Encode.Value) -> Cmd msg
port renderFinished : (Bool -> msg) -> Sub msg

main : Program Pipeline.Flags
main =
  Html.App.programWithFlags
    { init = Pipeline.init { render = renderPipeline, renderFinished = renderFinished }
    , update = Pipeline.update
    , view = Pipeline.view
    , subscriptions = Pipeline.subscriptions
    }
