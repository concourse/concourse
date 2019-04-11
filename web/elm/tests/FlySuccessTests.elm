module FlySuccessTests exposing (all)

import Application.Application as Application
import Common exposing (queryView)
import DashboardTests exposing (defineHoverBehaviour, iconSelector)
import Expect exposing (Expectation)
import Html.Attributes as Attr
import Http
import Message.Callback exposing (Callback(..))
import Message.Message
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


flags : Application.Flags
flags =
    { turbulenceImgSrc = ""
    , notFoundImgSrc = ""
    , csrfToken = ""
    , authToken = authToken
    , instanceName = ""
    , pipelineRunningKeyframes = ""
    }



-- SETUPS (i dunno, maybe use fuzzers?)


type alias SetupSteps =
    () -> Application.Model


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
                , query = Just "fly_port=1234"
                , fragment = Nothing
                }
                |> Tuple.first
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
                |> Tuple.first
        )


tokenSendSuccess : Setup
tokenSendSuccess =
    makeSetup "when token successfully sent to fly"
        (steps whenOnFlySuccessPage
            >> Application.handleCallback
                (TokenSentToFly (Ok ()))
            >> Tuple.first
        )


tokenSendFailed : Setup
tokenSendFailed =
    makeSetup "when token failed to send to fly"
        (steps whenOnFlySuccessPage
            >> Application.handleCallback
                (TokenSentToFly (Err Http.NetworkError))
            >> Tuple.first
        )


tokenCopied : Setup
tokenCopied =
    makeSetup "when token copied to clipboard"
        (steps tokenSendFailed
            >> Application.update
                (Msgs.Update <| Message.Message.CopyToken)
            >> Tuple.first
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


button : Query
button =
    body >> Query.find [ id "copy-token" ]


buttonIcon : Query
buttonIcon =
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
            >> query
            >> assertion



-- top bar


topBarProperty : Property
topBarProperty =
    property topBar "has bold font" <|
        Query.has [ style "font-weight" "700" ]



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
    property successCard "has smooth, non-bold font" <|
        Query.has
            [ style "-webkit-font-smoothing" "antialiased"
            , style "font-weight" "400"
            ]



-- title


titleText : Property
titleText =
    property title "has success text" <|
        Query.has [ text "login successful!" ]


titleStyle : Property
titleStyle =
    property title "has bold 18px font" <|
        Query.has
            [ style "font-size" "18px"
            , style "font-weight" "700"
            ]


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


buttonStyleUnclicked : Property
buttonStyleUnclicked =
    property button "display inline and has almost-white border" <|
        Query.has
            [ tag "span"
            , style "border" <| "1px solid " ++ almostWhite
            ]


buttonStyleClicked : Property
buttonStyleClicked =
    property button "has blue border and background" <|
        Query.has
            [ style "background-color" blue
            , style "border" <| "1px solid " ++ blue
            ]


buttonSize : Property
buttonSize =
    property button "is 212px wide with 10px padding above and below" <|
        Query.has
            [ style "width" "212px"
            , style "padding" "10px 0"
            ]


buttonPosition : Property
buttonPosition =
    property button "has 15px margin above and below" <|
        Query.has [ style "margin" "15px 0" ]


buttonLayout : Property
buttonLayout =
    property button "lays out contents horizontally, centering" <|
        Query.has
            [ style "display" "flex"
            , style "justify-content" "center"
            , style "align-items" "center"
            ]


buttonTextPrompt : Property
buttonTextPrompt =
    property button "says 'copy token to clipboard'" <|
        Query.has [ text "copy token to clipboard" ]


buttonTextClicked : Property
buttonTextClicked =
    property button "says 'token copied'" <|
        Query.has [ text "token copied" ]


buttonCursorUnclicked : Property
buttonCursorUnclicked =
    property button "has pointer cursor" <|
        Query.has [ style "cursor" "pointer" ]


buttonCursorClicked : Property
buttonCursorClicked =
    property button "has default cursor" <|
        Query.has [ style "cursor" "default" ]


buttonClipboardAttr : Property
buttonClipboardAttr =
    property button "has attribute that is readable by clipboard.js" <|
        Query.has
            [ attribute <|
                Attr.attribute
                    "data-clipboard-text"
                    authToken
            ]


buttonClickHandler : Property
buttonClickHandler =
    property button "sends CopyToken on click" <|
        Event.simulate Event.click
            >> Event.expect
                (Msgs.Update <| Message.Message.CopyToken)



-- icon


iconStyle : Property
iconStyle =
    property buttonIcon "has clipboard icon" <|
        Query.has <|
            iconSelector { size = "20px", image = "clippy.svg" }


iconPosition : Property
iconPosition =
    property buttonIcon "has margin on the right" <|
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
        [ describe "top bar" <| tests allCases [ topBarProperty ]
        , describe "card" cardTests
        , describe "title" titleTests
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
            , whenOnFlySuccessPage |> bodyPendingText
            , whenOnFlySuccessPage |> bodyNoButton
            , tokenSendSuccess |> firstParagraphSuccessText
            , tokenSendSuccess |> secondParagraphSuccessText
            ]
        , describe "button"
            [ describe "always" <|
                tests [ tokenSendFailed, tokenCopied ]
                    [ buttonSize
                    , buttonPosition
                    , buttonLayout
                    , buttonClipboardAttr
                    , firstParagraphFailureText
                    , secondParagraphFailureText
                    ]
            , describe "when token sending failed" <|
                tests [ tokenSendFailed ]
                    [ buttonStyleUnclicked
                    , buttonClickHandler
                    , buttonTextPrompt
                    , iconStyle
                    , iconPosition
                    , buttonCursorUnclicked
                    ]
            , describe "after copying token" <|
                tests [ tokenCopied ]
                    [ buttonStyleClicked
                    , buttonTextClicked
                    , iconStyle
                    , iconPosition
                    , buttonCursorClicked
                    ]
            , defineHoverBehaviour
                { name = "copy token button"
                , setup = steps tokenSendFailed ()
                , query = button
                , updateFunc = \msg -> Application.update msg >> Tuple.first
                , unhoveredSelector =
                    { description =
                        "same background as card"
                    , selector = [ style "background-color" darkGrey ]
                    }
                , mouseEnterMsg =
                    Msgs.Update <|
                        Message.Message.Hover <|
                            Just Message.Message.CopyTokenButton
                , mouseLeaveMsg =
                    Msgs.Update <|
                        Message.Message.Hover Nothing
                , hoveredSelector =
                    { description = "darker background"
                    , selector =
                        [ style "background-color" darkerGrey ]
                    }
                }
            ]
        ]
