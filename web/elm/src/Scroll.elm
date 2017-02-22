effect module Scroll
    where { subscription = MySub }
    exposing
        ( FromBottom
        , toBottom
        , toWindowBottom
        , fromWindowBottom
        , scroll
        , scrollIntoView
        )

import Dom.LowLevel as Dom
import Json.Decode as Json
import Process
import Task exposing (Task)
import Native.Scroll


type alias FromBottom =
    Int


decodeFromBottom : Json.Decoder FromBottom
decodeFromBottom =
    customDecoder decodeComparators <|
        \( yOffset, clientHeight, scrollHeight ) ->
            let
                scrolledHeight =
                    yOffset + clientHeight
            in
                Ok (scrollHeight - scrolledHeight)


decodeComparators : Json.Decoder ( Int, Int, Int )
decodeComparators =
    Json.map3 (,,)
        (Json.at [ "currentTarget", "pageYOffset" ] Json.int)
        (Json.at [ "target", "documentElement", "clientHeight" ] Json.int)
        (Json.at [ "target", "documentElement", "scrollHeight" ] Json.int)



-- SCROLL EVENTS


fromWindowBottom : (FromBottom -> msg) -> Sub msg
fromWindowBottom tagger =
    subscription (MySub tagger)


toBottom : String -> Task x ()
toBottom =
    Native.Scroll.toBottom


toWindowBottom : Task x ()
toWindowBottom =
    Native.Scroll.toWindowBottom ()


scroll : String -> Float -> Task x ()
scroll =
    Native.Scroll.scrollElement


scrollIntoView : String -> Task x ()
scrollIntoView =
    Native.Scroll.scrollIntoView



-- SUBSCRIPTIONS


type MySub msg
    = MySub (FromBottom -> msg)


subMap : (a -> b) -> MySub a -> MySub b
subMap func (MySub tagger) =
    MySub (tagger >> func)



-- EFFECT MANAGER


type alias State msg =
    Maybe
        { subs : List (MySub msg)
        , watcher : Process.Id
        }


init : Task Never (State msg)
init =
    Task.succeed Nothing


onEffects : Platform.Router msg FromBottom -> List (MySub msg) -> State msg -> Task Never (State msg)
onEffects router newSubs state =
    case ( state, newSubs ) of
        ( Nothing, [] ) ->
            Task.succeed Nothing

        ( Just { watcher }, [] ) ->
            Process.kill watcher
                |> Task.andThen
                    (\_ ->
                        Task.succeed Nothing
                    )

        ( Nothing, _ ) ->
            Process.spawn (Dom.onWindow "scroll" decodeFromBottom (Platform.sendToSelf router))
                |> Task.andThen
                    (\watcher ->
                        Task.succeed (Just { subs = newSubs, watcher = watcher })
                    )

        ( Just state, _ ) ->
            Task.succeed (Just { state | subs = newSubs })


onSelfMsg : Platform.Router msg FromBottom -> FromBottom -> State msg -> Task Never (State msg)
onSelfMsg router fb state =
    case state of
        Nothing ->
            Task.succeed Nothing

        Just { subs } ->
            let
                send (MySub tagger) =
                    Platform.sendToApp router (tagger fb)
            in
                Task.sequence (List.map send subs)
                    |> Task.andThen
                        (\_ ->
                            Task.succeed state
                        )


customDecoder : Json.Decoder b -> (b -> Result String a) -> Json.Decoder a
customDecoder decoder toResult =
    Json.andThen
        (\a ->
            case toResult a of
                Ok b ->
                    Json.succeed b

                Err err ->
                    Json.fail err
        )
        decoder
