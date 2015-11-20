module Main where

import Html exposing (Html)
import Effects
import StartApp
import Task exposing (Task)

import Build

port buildId : Int

app : StartApp.App Build.Model
app =
  let
    pageDrivenActions =
      Signal.mailbox Build.Noop
  in
    StartApp.start
      { init = Build.init pageDrivenActions.address buildId
      , update = Build.update
      , view = Build.view
      , inputs = [pageDrivenActions.signal]
      }

main : Signal Html
main =
  app.html

port tasks : Signal (Task Effects.Never ())
port tasks =
  app.tasks
