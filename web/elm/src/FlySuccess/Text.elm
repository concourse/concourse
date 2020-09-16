module FlySuccess.Text exposing
    ( Paragraph
    , copyTokenButton
    , copyTokenInput
    , firstParagraph
    , flyLoginLinkDescription
    , flyLoginLinkText
    , pending
    , secondParagraph
    , sendTokenButton
    , title
    )

import FlySuccess.Models as Models exposing (ButtonState)


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


firstParagraph : Models.TokenTransfer -> Paragraph
firstParagraph tokenTransfer =
    case tokenTransfer of
        Models.Pending ->
            []

        Models.Success ->
            [ "your token has been transferred to fly." ]

        Models.NetworkTrouble ->
            [ "however, your token could not be"
            , "sent to fly."
            ]

        Models.BlockedByBrowser ->
            [ "however, your token could not be sent"
            , "to fly because your browser blocked"
            , "the attempt."
            ]

        Models.NoFlyPort ->
            [ "however, your token could not be"
            , "sent to fly."
            ]


secondParagraph : Models.TokenTransfer -> Paragraph
secondParagraph error =
    case error of
        Models.Pending ->
            []

        Models.Success ->
            [ "you may now close this window." ]

        Models.NetworkTrouble ->
            [ "after copying, return to fly and paste"
            , "your token into the prompt."
            ]

        Models.BlockedByBrowser ->
            [ "if that fails, you will need to copy"
            , "the token to your clipboard, return"
            , "to fly, and paste your token into"
            , "the prompt."
            ]

        Models.NoFlyPort ->
            [ "could not find a valid fly port to send to."
            , "maybe your URL is broken?"
            ]


copyTokenButton : ButtonState -> String
copyTokenButton buttonState =
    if Models.isClicked buttonState then
        "token copied"

    else
        "copy token to clipboard"


copyTokenInput : String
copyTokenInput =
    "copy token here"


sendTokenButton : String
sendTokenButton =
    "send token to fly directly"


flyLoginLinkDescription : Line
flyLoginLinkDescription =
    "Or try manually sending the token to fly:"


flyLoginLinkText : Line
flyLoginLinkText =
    "send token to fly directly"
