module Message.SubPageMsgs exposing (Msg(..))

import Message.BuildMsgs
import Message.DashboardMsgs
import Message.FlySuccessMsgs
import Message.JobMsgs
import Message.NotFoundMsgs
import Message.PipelineMsgs
import Message.ResourceMsgs


type Msg
    = BuildMsg Message.BuildMsgs.Msg
    | JobMsg Message.JobMsgs.Msg
    | ResourceMsg Message.ResourceMsgs.Msg
    | PipelineMsg Message.PipelineMsgs.Msg
    | DashboardMsg Message.DashboardMsgs.Msg
    | FlySuccessMsg Message.FlySuccessMsgs.Msg
    | NotFoundMsg Message.NotFoundMsgs.Msg
