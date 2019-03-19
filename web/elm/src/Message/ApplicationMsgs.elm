module Message.ApplicationMsgs exposing (Msg(..))

import Message.Callback exposing (Callback)
import Message.Message
import Message.Subscription exposing (Delivery)
import Routes


type Msg
    = RouteChanged Routes.Route
    | SubMsg Message.Message.Message
    | ModifyUrl Routes.Route
    | Callback Callback
    | DeliveryReceived Delivery
