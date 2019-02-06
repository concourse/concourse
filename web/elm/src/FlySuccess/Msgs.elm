module FlySuccess.Msgs exposing (Msg(..))

import NewTopBar.Msgs


type Msg
    = CopyTokenButtonHover Bool
    | CopyToken
    | FromTopBar NewTopBar.Msgs.Msg
