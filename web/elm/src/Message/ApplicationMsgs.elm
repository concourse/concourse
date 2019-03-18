module Message.ApplicationMsgs exposing (Msg(..), NavIndex)

import Message.Callback exposing (Callback)
import Message.Effects as Effects
import Message.SubPageMsgs
import Message.Subscription exposing (Delivery)
import Routes


type alias NavIndex =
    Int


type Msg
    = RouteChanged Routes.Route
    | SubMsg NavIndex Message.SubPageMsgs.Msg
    | ModifyUrl Routes.Route
    | Callback Effects.LayoutDispatch Callback
    | DeliveryReceived Delivery
