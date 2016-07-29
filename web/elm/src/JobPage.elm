port module JobPage exposing (..)

import Html.App
import Time

import Job

main =
  Html.App.programWithFlags
    { init = Job.init
    , update = Job.update
    , view = Job.view
    , subscriptions = always (Time.every Time.second Job.ClockTick)
    }
