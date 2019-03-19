module Message.ApplicationMsgs exposing (Msg(..), NavIndex)

import Message.Callback exposing (Callback)
import Message.Effects as Effects
import Message.Message
import Message.Subscription exposing (Delivery)
import Routes


type alias NavIndex =
    Int


type Msg
    = RouteChanged Routes.Route
    | SubMsg NavIndex Message.Message.Message
    | ModifyUrl Routes.Route
    | Callback Effects.LayoutDispatch Callback
    | DeliveryReceived Delivery
