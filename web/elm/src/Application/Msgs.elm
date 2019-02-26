module Application.Msgs exposing (Delivery(..), Interval(..), Msg(..), NavIndex, intervalToTime)

import Callback exposing (Callback)
import Effects
import Keyboard
import Routes
import SubPage.Msgs
import Time


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
-- Everything below here needs to be moved to Subscriptions! Don't leave them here!


type Delivery
    = KeyDown Keyboard.KeyCode
    | KeyUp Keyboard.KeyCode
    | MouseMoved
    | MouseClicked
    | ClockTicked Interval Time.Time
    | AnimationFrameAdvanced


type Interval
    = OneSecond
    | FiveSeconds
    | OneMinute


intervalToTime : Interval -> Time.Time
intervalToTime t =
    case t of
        OneSecond ->
            Time.second

        FiveSeconds ->
            5 * Time.second

        OneMinute ->
            Time.minute
