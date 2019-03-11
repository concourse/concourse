module Application.Msgs exposing (Msg(..), NavIndex)

import Callback exposing (Callback)
import Effects
import Routes
import SubPage.Msgs
import Subscription exposing (Delivery)


type alias NavIndex =
    Int


type Msg
    = RouteChanged Routes.Route
    | SubMsg NavIndex SubPage.Msgs.Msg
    | ModifyUrl Routes.Route
    | Callback Effects.LayoutDispatch Callback
    | DeliveryReceived Delivery
