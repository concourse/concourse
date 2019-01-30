module Msgs exposing (Msg(..), NavIndex)

import Callback exposing (Callback)
import Effects
import Routes
import SubPage.Msgs
import TopBar.Msgs


type alias NavIndex =
    Int


type Msg
    = RouteChanged Routes.ConcourseRoute
    | SubMsg NavIndex SubPage.Msgs.Msg
    | TopMsg NavIndex TopBar.Msgs.Msg
    | NewUrl String
    | ModifyUrl String
    | TokenReceived (Maybe String)
    | Callback Effects.LayoutDispatch Callback
