port module JobPage exposing (main)

import Html.App
import Time

import Job

port selectJobGroups : List String -> Cmd msg

main : Program Job.Flags
main =
  Html.App.programWithFlags
    { init = Job.init { selectGroups = selectJobGroups }
    , update = Job.update
    , view = Job.view
    , subscriptions = always (Time.every Time.second Job.ClockTick)
    }
