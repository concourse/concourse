module SubPage.Msgs exposing (Msg(..))

import Autoscroll
import Build.Msgs
import Concourse
import Dashboard.Msgs
import Effects
import FlySuccess.Msgs
import Http
import Job.Msgs
import Pipeline.Msgs
import Resource.Msgs


type Msg
    = BuildMsg (Autoscroll.Msg Build.Msgs.Msg)
    | JobMsg Job.Msgs.Msg
    | ResourceMsg Resource.Msgs.Msg
    | PipelineMsg Pipeline.Msgs.Msg
    | NewCSRFToken String
    | DashboardPipelinesFetched (Result Http.Error (List Concourse.Pipeline))
    | DashboardMsg Dashboard.Msgs.Msg
    | FlySuccessMsg FlySuccess.Msgs.Msg
    | Callback Effects.Callback
    | CallbackAutoScroll (Autoscroll.Msg Effects.Callback)
