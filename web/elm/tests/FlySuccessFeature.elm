module FlySuccessFeature exposing (all)

import Application.Application as Application
import Assets
import Common exposing (defineHoverBehaviour, queryView)
import DashboardTests exposing (iconSelector)
import Expect exposing (Expectation)
import Html.Attributes as Attr
import Http
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects
import Message.Message
import Message.Subscription as Subscription
import Message.TopLevelMessage as Msgs
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector
    exposing
        ( attribute
        , containing
        , id
        , style
        , tag
        , text
        )
import Url
import Views.Styles



-- CONSTANTS (might be able to remove this and refer to "configuration"-type
-- files like Colors.elm)


almostWhite : String
almostWhite =
    "#e6e7e8"


darkGrey : String
darkGrey =
    "#323030"


darkerGrey : String
darkerGrey =
    "#242424"


blue : String
blue =
    "#196ac8"


authToken : String
authToken =
    "some_auth_token"


flyPort : Int
flyPort =
    1234


flags : Application.Flags
flags =
    { turbulenceImgSrc = ""
    , notFoundImgSrc = ""
    , csrfToken = ""
    , authToken = authToken
    , pipelineRunningKeyframes = ""
    }



-- SETUPS (i dunno, maybe use fuzzers?)


type alias SetupSteps =
    () -> ( Application.Model, List Effects.Effect )


type alias Setup =
    ( String, SetupSteps )


setupDesc : Setup -> String
setupDesc =
    Tuple.first


steps : Setup -> SetupSteps
steps =
    Tuple.second


makeSetup : String -> SetupSteps -> Setup
makeSetup =
    \a b -> ( a, b )


whenOnFlySuccessPage : Setup
whenOnFlySuccessPage =
    makeSetup "when on fly success page"
        (\_ ->
            Application.init
                flags
                { protocol = Url.Http
                , host = ""
                , port_ = Nothing
                , path = "/fly_success"
                , query = Just <| "fly_port=" ++ String.fromInt flyPort
                , fragment = Nothing
                }
        )


whenOnNoopPage : Setup
whenOnNoopPage =
    makeSetup "when on fly success page with noop parameter"
        (\_ ->
            Application.init
                flags
                { protocol = Url.Http
                , host = ""
                , port_ = Nothing
                , path = "/fly_success"
                , query = Just <| "noop=true&fly_port=" ++ String.fromInt flyPort
                , fragment = Nothing
                }
        )


invalidFlyPort : Setup
invalidFlyPort =
    makeSetup "with invalid fly port"
        (\_ ->
            Application.init
                flags
                { protocol = Url.Http
                , host = ""
                , port_ = Nothing
                , path = "/fly_success"
                , query = Just "fly_port=banana"
                , fragment = Nothing
                }
        )


tokenSendSuccess : Setup
tokenSendSuccess =
    makeSetup "when token successfully sent to fly"
        (steps whenOnFlySuccessPage
            >> Tuple.first
            >> Application.handleDelivery
                (Subscription.TokenSentToFly Subscription.Success)
        )


tokenSendFailed : Setup
tokenSendFailed =
    makeSetup "when token failed to send to fly"
        (steps whenOnFlySuccessPage
            >> Tuple.first
            >> Application.handleDelivery
                (Subscription.TokenSentToFly Subscription.NetworkError)
        )


tokenSendBlocked : Setup
tokenSendBlocked =
    makeSetup "when token sending is blocked by the browser"
        (steps whenOnFlySuccessPage
            >> Tuple.first
            >> Application.handleDelivery
                (Subscription.TokenSentToFly Subscription.BrowserError)
        )


tokenCopied : Setup
tokenCopied =
    makeSetup "when token copied to clipboard"
        (steps tokenSendFailed
            >> Tuple.first
            >> Application.update
                (Msgs.Update <|
                    Message.Message.Click Message.Message.CopyTokenButton
                )
        )


allCases : List Setup
allCases =
    [ whenOnFlySuccessPage
    , invalidFlyPort
    , tokenSendFailed
    , tokenSendSuccess
    ]



-- QUERIES


type alias Query =
    Application.Model -> Query.Single Msgs.TopLevelMessage


topBar : Query
topBar =
    queryView >> Query.find [ id "top-bar-app" ]


successCard : Query
successCard =
    queryView >> Query.find [ id "success-card" ]


title : Query
title =
    successCard >> Query.find [ id "success-card-title" ]


body : Query
body =
    successCard >> Query.find [ id "success-card-body" ]


firstParagraph : Query
firstParagraph =
    successCard
        >> Query.find [ id "success-card-body" ]
        >> Query.find [ id "first-paragraph" ]


secondParagraph : Query
secondParagraph =
    successCard
        >> Query.find [ id "success-card-body" ]
        >> Query.find [ id "second-paragraph" ]


copyTokenButton : Query
copyTokenButton =
    body >> Query.find [ id "copy-token" ]


copyTokenInput : Query
copyTokenInput =
    body >> Query.find [ id "manual-copy-token" ]


sendTokenButton : Query
sendTokenButton =
    body >> Query.find [ id "send-token" ]


copyTokenButtonIcon : Query
copyTokenButtonIcon =
    body
        >> Query.find [ id "copy-token" ]
        >> Query.find [ id "copy-icon" ]



-- PROPERTIES


type alias Assertion =
    Query.Single Msgs.TopLevelMessage -> Expectation


type alias Property =
    Setup -> Test


property : Query -> String -> Assertion -> Property
property query description assertion setup =
    test (setupDesc setup ++ ", " ++ description) <|
        steps setup
            >> Tuple.first
            >> query
            >> assertion



-- token send effect


sendsToken : Property
sendsToken setup =
    test (setupDesc setup ++ ", sends token to fly") <|
        steps setup
            >> Tuple.second
            >> Common.contains (Effects.SendTokenToFly authToken flyPort)


doesNotSendToken : Property
doesNotSendToken setup =
    test (setupDesc setup ++ ", does not send token to fly") <|
        steps setup
            >> Tuple.second
            >> Common.notContains (Effects.SendTokenToFly authToken flyPort)



-- subscription


listensForTokenResponse : Property
listensForTokenResponse setup =
    test (setupDesc setup ++ ", listens for token response") <|
        steps setup
            >> Tuple.first
            >> Application.subscriptions
            >> Common.contains Subscription.OnTokenSentToFly



-- card


cardProperties : List Property
cardProperties =
    [ cardBackground
    , cardSize
    , cardPosition
    , cardLayout
    , cardStyle
    ]


cardBackground : Property
cardBackground =
    property successCard "card has dark grey background" <|
        Query.has [ style "background-color" darkGrey ]


cardSize : Property
cardSize =
    property successCard "is 330px wide with 30px padding" <|
        Query.has [ style "padding" "30px", style "width" "330px" ]


cardPosition : Property
cardPosition =
    property successCard "is centered 50px from the top of the document" <|
        Query.has [ style "margin" "50px auto" ]


cardLayout : Property
cardLayout =
    property successCard "lays out contents vertically and center aligned" <|
        Query.has
            [ style "display" "flex"
            , style "flex-direction" "column"
            , style "align-items" "center"
            , style "text-align" "center"
            ]


cardStyle : Property
cardStyle =
    property successCard "has light font" <|
        Query.has [ style "font-weight" Views.Styles.fontWeightLight ]



-- title


titleText : Property
titleText =
    property title "has success text" <|
        Query.has [ text "login successful!" ]


titleStyle : Property
titleStyle =
    property title "has 18px font" <|
        Query.has
            [ style "font-size" "18px" ]


titleProperties : List Property
titleProperties =
    [ titleText
    , titleStyle
    ]



-- body


bodyPendingText : Property
bodyPendingText =
    property body "has pending text" <|
        Query.has [ text "sending token to fly..." ]


bodyNoButton : Property
bodyNoButton =
    property body "has no 'copy token' button" <|
        Query.hasNot [ id "copy-token" ]


bodyStyle : Property
bodyStyle =
    property body "has 14px font" <|
        Query.has [ style "font-size" "14px" ]


bodyPosition : Property
bodyPosition =
    property body "has 10px margin above and below" <|
        Query.has [ style "margin" "10px 0" ]


bodyLayout : Property
bodyLayout =
    property body "lays out contents vertically, centering horizontally" <|
        Query.has
            [ style "display" "flex"
            , style "flex-direction" "column"
            , style "align-items" "center"
            ]


bodyParagraphPositions : Property
bodyParagraphPositions =
    property body "paragraphs have 5px margin above and below" <|
        Query.findAll [ tag "p" ]
            >> Query.each (Query.has [ style "margin" "5px 0" ])



-- body on any type of failure


copyLinkInput : Property
copyLinkInput =
    property body "label gives instructions for manual copying" <|
        Query.children []
            >> Expect.all
                [ Query.index 1
                    >> Query.has
                        [ text "copy token here" ]
                , Query.index 1
                    >> Query.children [ tag "input" ]
                    >> Query.first
                    >> Query.has
                        [ attribute <| Attr.value authToken
                        , style "white-space" "nowrap"
                        , style "overflow" "hidden"
                        , style "text-overflow" "ellipsis"
                        ]
                ]



-- body on invalid fly port


secondParagraphErrorText : Property
secondParagraphErrorText =
    property secondParagraph "error message describes invalid fly port" <|
        Query.children []
            >> Expect.all
                [ Query.count (Expect.equal 3)
                , Query.index 0
                    >> Query.has
                        [ text "could not find a valid fly port to send to." ]
                , Query.index 2
                    >> Query.has
                        [ text "maybe your URL is broken?" ]
                ]



-- body on browser blocking token from sending


firstParagraphBlockedText : Property
firstParagraphBlockedText =
    property firstParagraph
        "explains that your browser blocked the token from sending"
    <|
        Query.children []
            >> Expect.all
                [ Query.count (Expect.equal 5)
                , Query.index 0
                    >> Query.has
                        [ text "however, your token could not be sent" ]
                , Query.index 2
                    >> Query.has
                        [ text "to fly because your browser blocked" ]
                , Query.index 4
                    >> Query.has
                        [ text "the attempt." ]
                ]


secondParagraphBlockedText : Property
secondParagraphBlockedText =
    property secondParagraph "describes copy-pasting option" <|
        Query.children []
            >> Expect.all
                [ Query.count (Expect.equal 7)
                , Query.index 0
                    >> Query.has
                        [ text "if that fails, you will need to copy" ]
                , Query.index 2
                    >> Query.has
                        [ text "the token to your clipboard, return" ]
                , Query.index 4
                    >> Query.has
                        [ text "to fly, and paste your token into" ]
                , Query.index 6
                    >> Query.has
                        [ text "the prompt." ]
                ]



-- body on successfully sending token


firstParagraphSuccessText : Property
firstParagraphSuccessText =
    property firstParagraph "says 'your token has been transferred to fly'" <|
        Query.has [ text "your token has been transferred to fly." ]


secondParagraphSuccessText : Property
secondParagraphSuccessText =
    property secondParagraph "says 'you may now close this window'" <|
        Query.has [ text "you may now close this window." ]



-- body on failing to send token


firstParagraphFailureText : Property
firstParagraphFailureText =
    property firstParagraph
        "says 'however, your token could not be sent to fly.'"
    <|
        Query.children []
            >> Expect.all
                [ Query.count (Expect.equal 3)
                , Query.index 0
                    >> Query.has
                        [ text "however, your token could not be" ]
                , Query.index 2 >> Query.has [ text "sent to fly." ]
                ]


pasteInstructions : Query -> Property
pasteInstructions query =
    property query
        ("says 'after copying, return to fly and paste your token "
            ++ "into the prompt.'"
        )
    <|
        Query.children []
            >> Expect.all
                [ Query.count (Expect.equal 3)
                , Query.index 0
                    >> Query.has
                        [ text "after copying, return to fly and paste" ]
                , Query.index 2
                    >> Query.has
                        [ text "your token into the prompt." ]
                ]


secondParagraphFailureText : Property
secondParagraphFailureText =
    property secondParagraph
        ("says 'after copying, return to fly and paste your token "
            ++ "into the prompt.'"
        )
    <|
        Query.children []
            >> Expect.all
                [ Query.count (Expect.equal 3)
                , Query.index 0
                    >> Query.has
                        [ text "after copying, return to fly and paste" ]
                , Query.index 2
                    >> Query.has
                        [ text "your token into the prompt." ]
                ]



-- button


copyTokenButtonStyleUnclicked : Property
copyTokenButtonStyleUnclicked =
    property copyTokenButton "display inline and has almost-white border" <|
        Query.has
            [ tag "span"
            , style "border" <| "1px solid " ++ almostWhite
            ]


sendTokenButtonStyle : Property
sendTokenButtonStyle =
    property sendTokenButton "display inline and has almost-white border" <|
        Query.has
            [ tag "a"
            , style "border" <| "1px solid " ++ almostWhite
            ]


buttonStyleClicked : Property
buttonStyleClicked =
    property copyTokenButton "has blue border and background" <|
        Query.has
            [ style "background-color" blue
            , style "border" <| "1px solid " ++ blue
            ]


buttonSize : Property
buttonSize =
    property copyTokenButton "is 212px wide with 10px padding above and below" <|
        Query.has
            [ style "width" "212px"
            , style "padding" "10px 0"
            ]


buttonPosition : Property
buttonPosition =
    property copyTokenButton "has 15px margin above and below" <|
        Query.has [ style "margin" "15px 0" ]


buttonLayout : Property
buttonLayout =
    property copyTokenButton "lays out contents horizontally, centering" <|
        Query.has
            [ style "display" "flex"
            , style "justify-content" "center"
            , style "align-items" "center"
            ]


sendTokenButtonText : Property
sendTokenButtonText =
    property sendTokenButton "says 'send token to fly directly'" <|
        Query.has [ text "send token to fly directly" ]


copyTokenButtonTextPrompt : Property
copyTokenButtonTextPrompt =
    property copyTokenButton "says 'copy token to clipboard'" <|
        Query.has [ text "copy token to clipboard" ]


copyTokenButtonTextClicked : Property
copyTokenButtonTextClicked =
    property copyTokenButton "says 'token copied'" <|
        Query.has [ text "token copied" ]


buttonCursorUnclicked : Property
buttonCursorUnclicked =
    property copyTokenButton "has pointer cursor" <|
        Query.has [ style "cursor" "pointer" ]


buttonCursorClicked : Property
buttonCursorClicked =
    property copyTokenButton "has default cursor" <|
        Query.has [ style "cursor" "default" ]


buttonClipboardAttr : Property
buttonClipboardAttr =
    property copyTokenButton "has attribute that is readable by clipboard.js" <|
        Query.has
            [ attribute <|
                Attr.attribute
                    "data-clipboard-text"
                    authToken
            ]


copyTokenButtonClickHandler : Property
copyTokenButtonClickHandler =
    property copyTokenButton "sends CopyToken on click" <|
        Event.simulate Event.click
            >> Event.expect
                (Msgs.Update <|
                    Message.Message.Click Message.Message.CopyTokenButton
                )


sendTokenButtonClickHandler : Property
sendTokenButtonClickHandler =
    property sendTokenButton "is a link to fly" <|
        Query.has
            [ attribute <|
                Attr.href <|
                    "http://127.0.0.1:1234/?token="
                        ++ authToken
            ]



-- icon


iconStyle : Property
iconStyle =
    property copyTokenButtonIcon "has clipboard icon" <|
        Query.has <|
            iconSelector { size = "20px", image = Assets.ClippyIcon }


iconPosition : Property
iconPosition =
    property copyTokenButtonIcon "has margin on the right" <|
        Query.has [ style "margin-right" "5px" ]



-- TESTS


tests : List Setup -> List Property -> List Test
tests setups properties =
    setups
        |> List.concatMap
            (\setup -> List.map ((|>) setup) properties)


cardTests : List Test
cardTests =
    tests allCases cardProperties


titleTests : List Test
titleTests =
    tests allCases titleProperties


all : Test
all =
    describe "Fly login success page"
        [ describe "page load"
            [ whenOnFlySuccessPage |> listensForTokenResponse ]
        , describe "card" cardTests
        , describe "title" titleTests
        , describe "token sending"
            [ whenOnFlySuccessPage |> sendsToken
            , whenOnNoopPage |> doesNotSendToken
            ]
        , describe "body"
            [ describe "style" <|
                tests allCases
                    [ bodyStyle
                    , bodyPosition
                    , bodyLayout
                    , bodyParagraphPositions
                    ]
            , invalidFlyPort |> firstParagraphFailureText
            , invalidFlyPort |> secondParagraphErrorText
            , invalidFlyPort |> copyLinkInput
            , tokenSendBlocked |> firstParagraphBlockedText
            , tokenSendBlocked |> secondParagraphBlockedText
            , tokenSendBlocked |> copyLinkInput
            , tokenSendFailed |> firstParagraphFailureText
            , tokenSendFailed |> secondParagraphFailureText
            , tokenSendFailed |> copyLinkInput
            , tokenCopied |> firstParagraphFailureText
            , tokenCopied |> secondParagraphFailureText
            , whenOnFlySuccessPage |> bodyPendingText
            , whenOnFlySuccessPage |> bodyNoButton
            , tokenSendSuccess |> firstParagraphSuccessText
            , tokenSendSuccess |> secondParagraphSuccessText
            ]
        , describe "copy token input"
            [ defineHoverBehaviour
                { name = "copy token input"
                , setup = steps tokenSendFailed () |> Tuple.first
                , query = copyTokenInput
                , unhoveredSelector =
                    { description =
                        "same background as card"
                    , selector = [ style "background-color" darkGrey ]
                    }
                , hoverable =
                    Message.Message.CopyTokenInput
                , hoveredSelector =
                    { description = "darker background"
                    , selector =
                        [ style "background-color" darkerGrey ]
                    }
                }
            ]
        , describe "copy token button"
            [ describe "always" <|
                tests [ tokenSendFailed, tokenCopied, tokenSendBlocked ]
                    [ buttonSize
                    , buttonPosition
                    , buttonLayout
                    , buttonClipboardAttr
                    ]
            , describe "when token sending failed" <|
                tests [ tokenSendFailed ]
                    [ copyTokenButtonStyleUnclicked
                    , copyTokenButtonClickHandler
                    , copyTokenButtonTextPrompt
                    , iconStyle
                    , iconPosition
                    , buttonCursorUnclicked
                    ]
            , describe "after copying token" <|
                tests [ tokenCopied ]
                    [ buttonStyleClicked
                    , copyTokenButtonTextClicked
                    , iconStyle
                    , iconPosition
                    , buttonCursorClicked
                    ]
            , defineHoverBehaviour
                { name = "copy token button"
                , setup = steps tokenSendFailed () |> Tuple.first
                , query = copyTokenButton
                , unhoveredSelector =
                    { description =
                        "same background as card"
                    , selector = [ style "background-color" darkGrey ]
                    }
                , hoverable =
                    Message.Message.CopyTokenButton
                , hoveredSelector =
                    { description = "darker background"
                    , selector =
                        [ style "background-color" darkerGrey ]
                    }
                }
            ]
        , describe "send token button"
            [ tokenSendBlocked |> sendTokenButtonStyle
            , tokenSendBlocked |> sendTokenButtonText
            , tokenSendBlocked |> sendTokenButtonClickHandler
            , defineHoverBehaviour
                { name = "send token button"
                , setup = steps tokenSendBlocked () |> Tuple.first
                , query = sendTokenButton
                , unhoveredSelector =
                    { description =
                        "same background as card"
                    , selector = [ style "background-color" darkGrey ]
                    }
                , hoverable =
                    Message.Message.SendTokenButton
                , hoveredSelector =
                    { description = "darker background"
                    , selector =
                        [ style "background-color" darkerGrey ]
                    }
                }
            ]
        ]
