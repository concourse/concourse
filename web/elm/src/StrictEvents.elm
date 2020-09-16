module StrictEvents exposing
    ( DeltaMode(..)
    , ScrollState
    , WheelEvent
    , onLeftClick
    , onLeftClickNoPreventDefault
    , onLeftClickOrShiftLeftClick
    , onLeftClickStopPropagation
    , onLeftMouseDown
    , onLeftMouseDownCapturing
    , onScroll
    , onWheel
    )

import Html
import Html.Events
import Json.Decode


type alias WheelEvent =
    { deltaX : Float
    , deltaY : Float
    , deltaMode : DeltaMode
    }


type alias ScrollState =
    { scrollHeight : Float
    , scrollTop : Float
    , clientHeight : Float
    }


type DeltaMode
    = DeltaModePixel
    | DeltaModeLine
    | DeltaModePage


onLeftClick : msg -> Html.Attribute msg
onLeftClick msg =
    onLeftClickCapturing True False (Json.Decode.succeed ()) (always msg)


onLeftClickStopPropagation : msg -> Html.Attribute msg
onLeftClickStopPropagation msg =
    onLeftClickCapturing True True (Json.Decode.succeed ()) (always msg)


onLeftClickNoPreventDefault : msg -> Html.Attribute msg
onLeftClickNoPreventDefault msg =
    onLeftClickCapturing False False (Json.Decode.succeed ()) (always msg)


onLeftClickCapturing : Bool -> Bool -> Json.Decode.Decoder x -> (x -> msg) -> Html.Attribute msg
onLeftClickCapturing preventDefault stopPropagation captured msg =
    Html.Events.custom "click"
        (assertNoModifier
            |> Json.Decode.andThen
                (\_ ->
                    assertLeftButton
                        |> Json.Decode.andThen
                            (\_ ->
                                Json.Decode.map
                                    (\x ->
                                        { message = msg x
                                        , stopPropagation = stopPropagation
                                        , preventDefault = preventDefault
                                        }
                                    )
                                    captured
                            )
                )
        )


onLeftClickOrShiftLeftClick : msg -> msg -> Html.Attribute msg
onLeftClickOrShiftLeftClick msg shiftMsg =
    Html.Events.custom "click"
        (assertLeftButton
            |> Json.Decode.andThen
                (\_ ->
                    assertNo "ctrlKey"
                        |> Json.Decode.andThen
                            (\_ ->
                                assertNo "altKey"
                                    |> Json.Decode.andThen
                                        (\_ ->
                                            assertNo "metaKey"
                                                |> Json.Decode.andThen
                                                    (\_ ->
                                                        Json.Decode.map
                                                            (\x ->
                                                                { message = x
                                                                , stopPropagation = False
                                                                , preventDefault = True
                                                                }
                                                            )
                                                            (determineClickMsg
                                                                msg
                                                                shiftMsg
                                                            )
                                                    )
                                        )
                            )
                )
        )


onLeftMouseDown : msg -> Html.Attribute msg
onLeftMouseDown msg =
    onLeftMouseDownCapturing (Json.Decode.succeed ()) (always msg)


onLeftMouseDownCapturing : Json.Decode.Decoder x -> (x -> msg) -> Html.Attribute msg
onLeftMouseDownCapturing captured msg =
    Html.Events.custom "mousedown"
        (assertNoModifier
            |> Json.Decode.andThen
                (\_ ->
                    assertLeftButton
                        |> Json.Decode.andThen
                            (\_ ->
                                Json.Decode.map
                                    (\x ->
                                        { message = msg x
                                        , stopPropagation = False
                                        , preventDefault = True
                                        }
                                    )
                                    captured
                            )
                )
        )


onWheel : (WheelEvent -> msg) -> Html.Attribute msg
onWheel cons =
    Html.Events.custom "wheel"
        (Json.Decode.map
            (\x ->
                { message = cons x
                , stopPropagation = False
                , preventDefault = True
                }
            )
            decodeWheelEvent
        )


onScroll : (ScrollState -> msg) -> Html.Attribute msg
onScroll cons =
    Html.Events.on "scroll" <|
        Json.Decode.map cons decodeScrollEvent


determineClickMsg : msg -> msg -> Json.Decode.Decoder msg
determineClickMsg clickMsg shiftClickMsg =
    customDecoder (Json.Decode.field "shiftKey" Json.Decode.bool) <|
        \shiftKey ->
            if shiftKey then
                Ok shiftClickMsg

            else
                Ok clickMsg


assertNoModifier : Json.Decode.Decoder ()
assertNoModifier =
    assertNo "ctrlKey"
        |> Json.Decode.andThen
            (\_ ->
                assertNo "altKey"
                    |> Json.Decode.andThen
                        (\_ ->
                            assertNo "metaKey"
                                |> Json.Decode.andThen
                                    (\_ ->
                                        assertNo "shiftKey"
                                    )
                        )
            )


assertNo : String -> Json.Decode.Decoder ()
assertNo prop =
    customDecoder (Json.Decode.field prop Json.Decode.bool) <|
        \val ->
            if not val then
                Ok ()

            else
                Err (prop ++ " used - skipping")


assertLeftButton : Json.Decode.Decoder ()
assertLeftButton =
    customDecoder (Json.Decode.field "button" Json.Decode.int) <|
        \button ->
            if button == 0 then
                Ok ()

            else
                Err "not left button"


decodeWheelEvent : Json.Decode.Decoder WheelEvent
decodeWheelEvent =
    Json.Decode.map3 WheelEvent
        (Json.Decode.field "deltaX" Json.Decode.float)
        (Json.Decode.field "deltaY" Json.Decode.float)
        (Json.Decode.field "deltaMode" decodeDeltaMode)


decodeDeltaMode : Json.Decode.Decoder DeltaMode
decodeDeltaMode =
    Json.Decode.int
        |> Json.Decode.andThen
            (\mode ->
                case mode of
                    0 ->
                        Json.Decode.succeed DeltaModePixel

                    1 ->
                        Json.Decode.succeed DeltaModeLine

                    2 ->
                        Json.Decode.succeed DeltaModePage

                    _ ->
                        Json.Decode.fail <| "invalid deltaMode " ++ String.fromInt mode
            )


decodeScrollEvent : Json.Decode.Decoder ScrollState
decodeScrollEvent =
    Json.Decode.map3 ScrollState
        (Json.Decode.at [ "target", "scrollHeight" ] Json.Decode.float)
        (Json.Decode.at [ "target", "scrollTop" ] Json.Decode.float)
        (Json.Decode.at [ "target", "clientHeight" ] Json.Decode.float)


customDecoder : Json.Decode.Decoder b -> (b -> Result String a) -> Json.Decode.Decoder a
customDecoder decoder toResult =
    Json.Decode.andThen
        (\a ->
            case toResult a of
                Ok b ->
                    Json.Decode.succeed b

                Err err ->
                    Json.Decode.fail err
        )
        decoder
