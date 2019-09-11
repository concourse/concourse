module FlySuccess.Text exposing
    ( Paragraph
    , button
    , firstParagraphFailure
    , firstParagraphSuccess
    , pending
    , secondParagraphFailure
    , secondParagraphSuccess
    , title
    , flyLoginLinkText
    , flyLoginLinkDescription
    )

import FlySuccess.Models as Models
    exposing
        ( ButtonState
        , TransferFailure(..)
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


firstParagraphSuccess : Paragraph
firstParagraphSuccess =
    [ "your token has been transferred to fly." ]


secondParagraphSuccess : Paragraph
secondParagraphSuccess =
    [ "you may now close this window." ]


firstParagraphFailure : Paragraph
firstParagraphFailure =
    [ "however, your token could not be"
    , "sent to fly."
    ]


secondParagraphFailure : TransferFailure -> Paragraph
secondParagraphFailure error =
    case error of
        NetworkTrouble _ ->
            [ "after copying, return to fly and paste"
            , "your token into the prompt."
            ]

        NoFlyPort ->
            [ "could not find a valid fly port to send to."
            , "maybe your URL is broken?"
            ]


button : ButtonState -> String
button buttonState =
    if Models.isClicked buttonState then
        "token copied"

    else
        "copy token to clipboard"


flyLoginLinkDescription : Line
flyLoginLinkDescription =
    "Or try manually sending the token to fly:"

flyLoginLinkText : Line
flyLoginLinkText =
    "send token to fly directly"
