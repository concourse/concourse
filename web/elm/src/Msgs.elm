module Msgs exposing (Msg(..), NavIndex)

import Callback exposing (Callback)
import Effects
import Routes
import SubPage.Msgs


type alias NavIndex =
    Int


type Msg
    = RouteChanged Routes.Route
    | SubMsg NavIndex SubPage.Msgs.Msg
    | NewUrl String
    | ModifyUrl String
    | TokenReceived (Maybe String)
    | Callback Effects.LayoutDispatch Callback
