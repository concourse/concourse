module Message.FlySuccessMsgs exposing (Msg(..))

import Message.TopBarMsgs


type Msg
    = CopyTokenButtonHover Bool
    | CopyToken
    | FromTopBar Message.TopBarMsgs.Msg
