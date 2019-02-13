module FlySuccess.Msgs exposing (Msg(..))

import TopBar.Msgs


type Msg
    = CopyTokenButtonHover Bool
    | CopyToken
    | FromTopBar TopBar.Msgs.Msg
