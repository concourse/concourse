module FlySuccess.Text exposing
    ( Paragraph
    , button
    , firstParagraph
    , pending
    , secondParagraph
    , title
    )

import FlySuccess.Models as Models
    exposing
        ( ButtonState
        , TokenTransfer
        , TransferResult
        )


title : String
title =
    "login successful!"


pending : String
pending =
    "sending token to fly..."


type alias Line =
    String


type alias Paragraph =
    List Line


firstParagraph : Bool -> Paragraph
firstParagraph success =
    if success then
        [ "your token has been transferred to fly." ]

    else
        [ "however, your token could not be"
        , "sent to fly."
        ]


secondParagraph : TransferResult -> Paragraph
secondParagraph result =
    case result of
        Ok True ->
            [ "you may now close this window." ]

        Ok False ->
            [ "after copying, return to fly and paste"
            , "your token into the prompt."
            ]

        Err () ->
            [ "could not find a valid fly port to send to."
            , "maybe your URL is broken?"
            ]


button : ButtonState -> String
button buttonState =
    if Models.isClicked buttonState then
        "token copied"

    else
        "copy token to clipboard"
