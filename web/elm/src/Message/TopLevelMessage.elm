module Message.TopLevelMessage exposing (TopLevelMessage(..))

import Message.Callback exposing (Callback)
import Message.Message exposing (Message)
import Message.Subscription exposing (Delivery)


type TopLevelMessage
    = Update Message
    | Callback Callback
    | DeliveryReceived Delivery
