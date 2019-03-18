module Message.SubPageMsgs exposing (Msg(..))

import Message.BuildMsgs
import Message.DashboardMsgs
import Message.JobMsgs
import Message.Message
import Message.ResourceMsgs


type Msg
    = BuildMsg Message.BuildMsgs.Msg
    | JobMsg Message.JobMsgs.Msg
    | ResourceMsg Message.ResourceMsgs.Msg
    | PipelineMsg Message.Message.Message
    | DashboardMsg Message.DashboardMsgs.Msg
    | FlySuccessMsg Message.Message.Message
    | NotFoundMsg Message.Message.Message
