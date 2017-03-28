module StrictEvents
    exposing
        ( onLeftClick
        , onLeftClickOrShiftLeftClick
        , onLeftMouseDown
        , onLeftMouseDownCapturing
        , MouseWheelEvent
        , onMouseWheel
        , ScrollState
        , onScroll
        )

import Html
import Html.Events
import Json.Decode


type alias MouseWheelEvent =
    { deltaX : Float
    , deltaY : Float
    }


type alias ScrollState =
    { scrollHeight : Float
    , scrollTop : Float
    , clientHeight : Float
    }


onLeftClick : msg -> Html.Attribute msg
onLeftClick msg =
    onLeftClickCapturing (Json.Decode.succeed ()) (always msg)


onLeftClickCapturing : Json.Decode.Decoder x -> (x -> msg) -> Html.Attribute msg
onLeftClickCapturing captured msg =
    Html.Events.onWithOptions "click"
        { stopPropagation = False, preventDefault = True }
        (assertNoModifier
            |> Json.Decode.andThen
                (\_ ->
                    assertLeftButton
                        |> Json.Decode.andThen
                            (\_ ->
                                Json.Decode.map msg captured
                            )
                )
        )


onLeftClickOrShiftLeftClick : msg -> msg -> Html.Attribute msg
onLeftClickOrShiftLeftClick msg shiftMsg =
    Html.Events.onWithOptions "click"
        { stopPropagation = False, preventDefault = True }
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
                                                        determineClickMsg msg shiftMsg
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
    Html.Events.onWithOptions "mousedown"
        { stopPropagation = False, preventDefault = True }
        (assertNoModifier
            |> Json.Decode.andThen
                (\_ ->
                    assertLeftButton
                        |> Json.Decode.andThen
                            (\_ ->
                                Json.Decode.map msg captured
                            )
                )
        )


onMouseWheel : (MouseWheelEvent -> msg) -> Html.Attribute msg
onMouseWheel cons =
    Html.Events.onWithOptions "mousewheel"
        { stopPropagation = False, preventDefault = True }
        (Json.Decode.map cons decodeMouseWheelEvent)


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


decodeMouseWheelEvent : Json.Decode.Decoder MouseWheelEvent
decodeMouseWheelEvent =
    Json.Decode.map2 MouseWheelEvent
        (Json.Decode.field "deltaX" Json.Decode.float)
        (Json.Decode.field "deltaY" Json.Decode.float)


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
