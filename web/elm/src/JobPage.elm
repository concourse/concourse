module JobPage where

import Html exposing (Html)
import Effects
import StartApp
import Task exposing (Task)
import Time

import Job

port jobName : String
port pipelineName : String
port pageSince : Int
port pageUntil : Int

main : Signal Html
main =
  app.html

app : StartApp.App Job.Model
app =
  StartApp.start
    { init = Job.init redirects.address jobName pipelineName pageSince pageUntil
    , update = Job.update
    , view = Job.view
    , inputs = []
    , inits =
        [ Signal.map Job.ClockTick (Time.every Time.second)
        ]
    }

redirects : Signal.Mailbox String
redirects = Signal.mailbox ""

port redirect : Signal String
port redirect =
  redirects.signal

port tasks : Signal (Task Effects.Never ())
port tasks =
  app.tasks
