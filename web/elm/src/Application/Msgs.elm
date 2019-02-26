module Application.Msgs exposing (Delivery(..), Interval(..), Msg(..), NavIndex, intervalToTime)

import Callback exposing (Callback)
import Effects
import Keyboard
import Routes
import Scroll
import SubPage.Msgs
import Time
import Window


type alias NavIndex =
    Int


type Msg
    = RouteChanged Routes.Route
    | SubMsg NavIndex SubPage.Msgs.Msg
    | ModifyUrl Routes.Route
    | Callback Effects.LayoutDispatch Callback
    | DeliveryReceived Delivery



-- Everything below here needs to be moved to Subscriptions! Don't leave them here!


type Delivery
    = KeyDown Keyboard.KeyCode
    | KeyUp Keyboard.KeyCode
    | MouseMoved
    | MouseClicked
    | ClockTicked Interval Time.Time
    | AnimationFrameAdvanced
    | ScrolledFromWindowBottom Scroll.FromBottom
    | WindowResized Window.Size
    | NonHrefLinkClicked String
    | TokenReceived (Maybe String)



-- NonHrefLinkClicked must be a String because we can't parse it out too easily :(


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
