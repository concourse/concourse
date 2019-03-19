module Message.SubPageMsgs exposing (Msg(..))

import Message.Message


type Msg
    = BuildMsg Message.Message.Message
    | JobMsg Message.Message.Message
    | ResourceMsg Message.Message.Message
    | PipelineMsg Message.Message.Message
    | DashboardMsg Message.Message.Message
    | FlySuccessMsg Message.Message.Message
    | NotFoundMsg Message.Message.Message
