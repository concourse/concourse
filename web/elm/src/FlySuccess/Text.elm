module FlySuccess.Text exposing
    ( Paragraph
    , copyTokenButton
    , firstParagraphBlocked
    , firstParagraphFailure
    , firstParagraphSuccess
    , flyLoginLinkDescription
    , flyLoginLinkText
    , pending
    , secondParagraphFailure
    , secondParagraphSuccess
    , sendTokenButton
    , thirdParagraphBlocked
    , title
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


firstParagraphBlocked : Paragraph
firstParagraphBlocked =
    [ "however, your token could not be sent"
    , "to fly because your browser blocked"
    , "the attempt."
    ]


secondParagraphFailure : TransferFailure -> Paragraph
secondParagraphFailure error =
    case error of
        BlockedByBrowser ->
            [ "if that fails, you will need to copy"
            , "the token to your clipboard, return"
            , "to fly, and paste your token into"
            , "the prompt."
            ]

        NetworkTrouble ->
            [ "after copying, return to fly and paste"
            , "your token into the prompt."
            ]

        NoFlyPort ->
            [ "could not find a valid fly port to send to."
            , "maybe your URL is broken?"
            ]


thirdParagraphBlocked : Paragraph
thirdParagraphBlocked =
    [ "if that fails, click the button below:"
    ]


copyTokenButton : ButtonState -> String
copyTokenButton buttonState =
    if Models.isClicked buttonState then
        "token copied"

    else
        "copy token to clipboard"


sendTokenButton : String
sendTokenButton =
    "send token to fly directly"


flyLoginLinkDescription : Line
flyLoginLinkDescription =
    "Or try manually sending the token to fly:"


flyLoginLinkText : Line
flyLoginLinkText =
    "send token to fly directly"
