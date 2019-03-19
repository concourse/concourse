module Message.SubPageMsgs exposing (Msg(..))

import Message.BuildMsgs
import Message.JobMsgs
import Message.Message


type Msg
    = BuildMsg Message.BuildMsgs.Msg
    | JobMsg Message.JobMsgs.Msg
    | ResourceMsg Message.Message.Message
    | PipelineMsg Message.Message.Message
    | DashboardMsg Message.Message.Message
    | FlySuccessMsg Message.Message.Message
    | NotFoundMsg Message.Message.Message
