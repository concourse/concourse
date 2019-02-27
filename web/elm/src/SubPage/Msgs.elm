module SubPage.Msgs exposing (Msg(..))

import Build.Msgs
import Dashboard.Msgs
import FlySuccess.Msgs
import Job.Msgs
import NotFound
import Pipeline.Msgs
import Resource.Msgs


type Msg
    = BuildMsg Build.Msgs.Msg
    | JobMsg Job.Msgs.Msg
    | ResourceMsg Resource.Msgs.Msg
    | PipelineMsg Pipeline.Msgs.Msg
    | DashboardMsg Dashboard.Msgs.Msg
    | FlySuccessMsg FlySuccess.Msgs.Msg
    | NotFoundMsg NotFound.Msg
