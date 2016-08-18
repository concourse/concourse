module JobPage exposing (main)

import Html.App
import Time

import Job

main : Program Job.Flags
main =
  Html.App.programWithFlags
    { init = Job.init
    , update = Job.update
    , view = Job.view
    , subscriptions = always (Time.every Time.second Job.ClockTick)
    }
