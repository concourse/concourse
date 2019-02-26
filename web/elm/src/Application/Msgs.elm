module Application.Msgs exposing (Delivery(..), Msg(..), NavIndex)

import Callback exposing (Callback)
import Effects
import Keyboard
import Routes
import SubPage.Msgs


type alias NavIndex =
    Int


type Msg
    = RouteChanged Routes.Route
    | SubMsg NavIndex SubPage.Msgs.Msg
    | NewUrl String
    | ModifyUrl Routes.Route
    | TokenReceived (Maybe String)
    | Callback Effects.LayoutDispatch Callback
    | DeliveryReceived Delivery



-- NewUrl must be a String because of the subscriptions, and nasty type-contravariance. :(


type Delivery
    = KeyDown Keyboard.KeyCode
    | KeyUp Keyboard.KeyCode
    | MouseMoved
    | MouseClicked
