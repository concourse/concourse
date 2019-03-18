module Application.Msgs exposing (Msg(..), NavIndex)

import Message.Callback exposing (Callback)
import Message.Effects as Effects
import Message.Subscription exposing (Delivery)
import Routes
import SubPage.Msgs


type alias NavIndex =
    Int


type Msg
    = RouteChanged Routes.Route
    | SubMsg NavIndex SubPage.Msgs.Msg
    | ModifyUrl Routes.Route
    | Callback Effects.LayoutDispatch Callback
    | DeliveryReceived Delivery
