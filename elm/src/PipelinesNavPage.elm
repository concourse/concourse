port module PipelinesNavPage exposing (..)

import Html.App
import Mouse

import PipelinesNav exposing (Action (..), isDragging)

main : Program Never
main =
  Html.App.program
    { init = PipelinesNav.init
    , update = PipelinesNav.update
    , view = PipelinesNav.view
    , subscriptions =
        ( \model ->
            if isDragging model then
              Sub.batch [ Mouse.moves Drag, Mouse.ups StopDragging ]
            else Sub.none
        )
    }
