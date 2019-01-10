module FlySuccessTests exposing (all)

import DashboardTests exposing (defineHoverBehaviour, iconSelector)
import Expect exposing (Expectation)
import FlySuccess
import Html.Attributes as Attr
import Layout
import SubPage
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
    "#196AC8"


authToken : String
authToken =
    "some_auth_token"



-- SETUPS (i dunno, maybe use fuzzers?)


type alias SetupSteps =
    () -> Layout.Model


type alias Setup =
    ( String, SetupSteps )


setupDesc : Setup -> String
setupDesc =
    Tuple.first


steps : Setup -> SetupSteps
steps =
    Tuple.second


setup : String -> SetupSteps -> Setup
setup =
    (,)


whenOnFlySuccessPage : Setup
whenOnFlySuccessPage =
    setup "when on fly success page"
        (\_ ->
            Layout.init
                { turbulenceImgSrc = ""
                , notFoundImgSrc = ""
                , csrfToken = ""
                , authToken = authToken
                , pipelineRunningKeyframes = ""
                }
                { href = ""
                , host = ""
                , hostname = ""
                , protocol = ""
                , origin = ""
                , port_ = ""
                , pathname = "/fly_success"
                , search = "?fly_port=1234"
                , hash = ""
                , username = ""
                , password = ""
                }
                |> Tuple.first
        )


invalidFlyPort : Setup
invalidFlyPort =
    setup "with invalid fly port"
        (\_ ->
            Layout.init
                { turbulenceImgSrc = ""
                , notFoundImgSrc = ""
                , csrfToken = ""
                , authToken = authToken
                , pipelineRunningKeyframes = ""
                }
                { href = ""
                , host = ""
                , hostname = ""
                , protocol = ""
                , origin = ""
                , port_ = ""
                , pathname = "/fly_success"
                , search = "?fly_port=banana"
                , hash = ""
                , username = ""
                , password = ""
                }
                |> Tuple.first
        )


tokenSendSuccess : Setup
tokenSendSuccess =
    setup "when token successfully sent to fly"
        (steps whenOnFlySuccessPage
            >> Layout.update
                (Layout.SubMsg 1 <|
                    SubPage.FlySuccessMsg <|
                        FlySuccess.TokenSentToFly True
                )
            >> Tuple.first
        )


tokenSendFailed : Setup
tokenSendFailed =
    setup "when token failed to send to fly"
        (steps whenOnFlySuccessPage
            >> Layout.update
                (Layout.SubMsg 1 <|
                    SubPage.FlySuccessMsg <|
                        FlySuccess.TokenSentToFly False
                )
            >> Tuple.first
        )


tokenCopied : Setup
tokenCopied =
    setup "when token copied to clipboard"
        (steps tokenSendFailed
            >> Layout.update
                (Layout.SubMsg 1 <|
                    SubPage.FlySuccessMsg <|
                        FlySuccess.CopyToken
                )
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
    Layout.Model -> Query.Single Layout.Msg


topBar : Query
topBar =
    Layout.view
        >> Query.fromHtml
        >> Query.find [ id "top-bar-app" ]


successCard : Query
successCard =
    Layout.view
        >> Query.fromHtml
        >> Query.find [ id "success-card" ]


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
    Query.Single Layout.Msg -> Expectation


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
        Query.has [ style [ ( "font-weight", "700" ) ] ]



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
        Query.has [ style [ ( "background-color", darkGrey ) ] ]


cardSize : Property
cardSize =
    property successCard "is 330px wide with 30px padding" <|
        Query.has
            [ style
                [ ( "padding", "30px" )
                , ( "width", "330px" )
                ]
            ]


cardPosition : Property
cardPosition =
    property successCard "is centered 50px from the top of the document" <|
        Query.has [ style [ ( "margin", "50px auto" ) ] ]


cardLayout : Property
cardLayout =
    property successCard "lays out contents vertically and center aligned" <|
        Query.has
            [ style
                [ ( "display", "flex" )
                , ( "flex-direction", "column" )
                , ( "align-items", "center" )
                , ( "text-align", "center" )
                ]
            ]


cardStyle : Property
cardStyle =
    property successCard "has smooth, non-bold font" <|
        Query.has
            [ style
                [ ( "-webkit-font-smoothing", "antialiased" )
                , ( "font-weight", "400" )
                ]
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
            [ style
                [ ( "font-size", "18px" )
                , ( "font-weight", "700" )
                ]
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
        Query.has [ style [ ( "font-size", "14px" ) ] ]


bodyPosition : Property
bodyPosition =
    property body "has 10px margin above and below" <|
        Query.has [ style [ ( "margin", "10px 0" ) ] ]


bodyLayout : Property
bodyLayout =
    property body "lays out contents vertically, centering horizontally" <|
        Query.has
            [ style
                [ ( "display", "flex" )
                , ( "flex-direction", "column" )
                , ( "align-items", "center" )
                ]
            ]


bodyParagraphPositions : Property
bodyParagraphPositions =
    property body "paragraphs have 5px margin above and below" <|
        Query.findAll [ tag "p" ]
            >> Query.each
                (Query.has [ style [ ( "margin", "5px 0" ) ] ])



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
            , style [ ( "border", "1px solid " ++ almostWhite ) ]
            ]


buttonStyleClicked : Property
buttonStyleClicked =
    property button "has blue border and background" <|
        Query.has
            [ style
                [ ( "background-color", blue )
                , ( "border", "1px solid " ++ blue )
                ]
            ]


buttonSize : Property
buttonSize =
    property button "is 212px wide with 10px padding above and below" <|
        Query.has
            [ style
                [ ( "width", "212px" )
                , ( "padding", "10px 0" )
                ]
            ]


buttonPosition : Property
buttonPosition =
    property button "has 15px margin above and below" <|
        Query.has [ style [ ( "margin", "15px 0" ) ] ]


buttonLayout : Property
buttonLayout =
    property button "lays out contents horizontally, centering" <|
        Query.has
            [ style
                [ ( "display", "flex" )
                , ( "justify-content", "center" )
                , ( "align-items", "center" )
                ]
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
        Query.has [ style [ ( "cursor", "pointer" ) ] ]


buttonCursorClicked : Property
buttonCursorClicked =
    property button "has default cursor" <|
        Query.has [ style [ ( "cursor", "default" ) ] ]


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
                (Layout.SubMsg 1 <|
                    SubPage.FlySuccessMsg <|
                        FlySuccess.CopyToken
                )



-- icon


iconStyle : Property
iconStyle =
    property buttonIcon "has clipboard icon" <|
        Query.has <|
            iconSelector { size = "20px", image = "clippy.svg" }


iconPosition : Property
iconPosition =
    property buttonIcon "has margin on the right" <|
        Query.has [ style [ ( "margin-right", "5px" ) ] ]



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
                , updateFunc = \msg -> Layout.update msg >> Tuple.first
                , unhoveredSelector =
                    { description =
                        "same background as card"
                    , selector = [ style [ ( "background-color", darkGrey ) ] ]
                    }
                , mouseEnterMsg =
                    Layout.SubMsg 1 <|
                        SubPage.FlySuccessMsg <|
                            FlySuccess.CopyTokenButtonHover
                                True
                , mouseLeaveMsg =
                    Layout.SubMsg 1 <|
                        SubPage.FlySuccessMsg <|
                            FlySuccess.CopyTokenButtonHover
                                False
                , hoveredSelector =
                    { description = "darker background"
                    , selector =
                        [ style [ ( "background-color", darkerGrey ) ] ]
                    }
                }
            ]
        ]
