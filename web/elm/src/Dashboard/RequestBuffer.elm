module Dashboard.RequestBuffer exposing (Buffer(..), handleCallback, handleDelivery)

import EffectTransformer exposing (ET)
import Message.Callback exposing (Callback)
import Message.Effects exposing (Effect)
import Message.Subscription exposing (Delivery(..), Interval(..))


type Buffer a
    = Buffer Effect (Callback -> Bool) (a -> Bool) { get : a -> Bool, set : Bool -> a -> a }


handleDelivery : Delivery -> List (Buffer a) -> ET a
handleDelivery delivery =
    List.map (handleDeliverySingle delivery) >> List.foldl (>>) identity


handleDeliverySingle : Delivery -> Buffer a -> ET a
handleDeliverySingle delivery (Buffer effect _ isPaused shouldFire) ( model, effects ) =
    case delivery of
        ClockTicked FiveSeconds _ ->
            ( if isPaused model then
                shouldFire.set True model

              else
                shouldFire.set False model
            , if shouldFire.get model && not (isPaused model) then
                effect :: effects

              else
                effects
            )

        _ ->
            ( model, effects )


handleCallback : Callback -> List (Buffer a) -> ET a
handleCallback callback =
    List.map (handleCallbackSingle callback) >> List.foldl (>>) identity


handleCallbackSingle : Callback -> Buffer a -> ET a
handleCallbackSingle callback (Buffer _ callbackMatcher _ shouldFire) ( model, effects ) =
    ( if callbackMatcher callback then
        shouldFire.set True model

      else
        model
    , effects
    )
