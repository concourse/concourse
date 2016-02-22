module Main where

import Html exposing (Html)
import Effects
import StartApp
import Task exposing (Task)
import Time

import Autoscroll
import Build
import Scroll

port buildId : Int

main : Signal Html
main =
  app.html

app : StartApp.App (Autoscroll.Model Build.Model)
app =
  let
    pageDrivenActions =
      Signal.mailbox Build.Noop
  in
    StartApp.start
      { init =
          Autoscroll.init
            Build.shouldAutoscroll <|
            Build.init redirects.address pageDrivenActions.address buildId
      , update = Autoscroll.update Build.update
      , view = Autoscroll.view Build.view
      , inputs =
          [ Signal.map Autoscroll.SubAction pageDrivenActions.signal
          , Signal.merge
              (Signal.map Autoscroll.FromBottom Scroll.fromBottom)
              (Signal.map (always Autoscroll.ScrollDown) (Time.every (50 * Time.millisecond)))
          ]
      , inits = [Signal.map (Autoscroll.SubAction << Build.ClockTick) (Time.every Time.second)]
      }

redirects : Signal.Mailbox String
redirects = Signal.mailbox ""

port redirect : Signal String
port redirect =
  redirects.signal

port tasks : Signal (Task Effects.Never ())
port tasks =
  app.tasks
