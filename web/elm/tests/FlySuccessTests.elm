module FlySuccessTests exposing (..)

import DashboardTests exposing (defineHoverBehaviour)
import Expect
import FlySuccess
import Html.Attributes as Attr
import Layout
import SubPage
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Event as Event
import Test.Html.Selector exposing (attribute, containing, id, style, tag, text)


loginSuccessText : String
loginSuccessText =
    "you have successfully logged in!"


loginSuccessDetails : String
loginSuccessDetails =
    "return to fly OR go back to the Concourse dashboard"


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


all : Test
all =
    describe "Fly login success card" <|
        let
            setup : () -> Layout.Model
            setup _ =
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
                    , search = ""
                    , hash = ""
                    , username = ""
                    , password = ""
                    }
                    |> Tuple.first

            successCard : Layout.Model -> Query.Single Layout.Msg
            successCard =
                Layout.view
                    >> Query.fromHtml
                    >> Query.find [ id "success-card" ]
        in
            [ test "has dark grey background" <|
                setup
                    >> successCard
                    >> Query.has [ style [ ( "background-color", darkGrey ) ] ]
            , test "has padding" <|
                setup
                    >> successCard
                    >> Query.has [ style [ ( "padding", "30px 20px" ) ] ]
            , test "has margin" <|
                setup
                    >> successCard
                    >> Query.has [ style [ ( "margin", "20px 30px" ) ] ]
            , test "lays out contents vertically, left-aligned" <|
                setup
                    >> successCard
                    >> Query.has
                        [ style
                            [ ( "display", "flex" )
                            , ( "flex-direction", "column" )
                            , ( "align-items", "flex-start" )
                            ]
                        ]
            , describe "success message" <|
                let
                    successMessage : Layout.Model -> Query.Single Layout.Msg
                    successMessage =
                        successCard >> Query.find [ id "success-message" ]
                in
                    [ test "has success text" <|
                        setup
                            >> successMessage
                            >> Query.has [ text loginSuccessText ]
                    , test "has 18px font size" <|
                        setup
                            >> successMessage
                            >> Query.has [ style [ ( "font-size", "18px" ) ] ]
                    , test "has no margin" <|
                        setup
                            >> successMessage
                            >> Query.has [ style [ ( "margin", "0" ) ] ]
                    ]
            , describe "success details" <|
                let
                    successDetails : Layout.Model -> Query.Single Layout.Msg
                    successDetails =
                        successCard >> Query.find [ id "success-details" ]
                in
                    [ test "has success text" <|
                        setup
                            >> successDetails
                            >> Query.has [ text loginSuccessDetails ]
                    , test "has 14px font size" <|
                        setup
                            >> successDetails
                            >> Query.has [ style [ ( "font-size", "14px" ) ] ]
                    , test "has 10px margin above and below" <|
                        setup
                            >> successDetails
                            >> Query.has [ style [ ( "margin", "10px 0" ) ] ]
                    ]
            , describe "copy token button" <|
                let
                    copyTokenButton : Layout.Model -> Query.Single Layout.Msg
                    copyTokenButton =
                        successCard >> Query.find [ id "copy-token" ]
                in
                    [ test "has white border" <|
                        setup
                            >> copyTokenButton
                            >> Query.has [ style [ ( "border", "1px solid " ++ almostWhite ) ] ]
                    , test "displays inline" <|
                        setup
                            >> copyTokenButton
                            >> Query.has [ tag "span" ]
                    , test "says 'copy token to clipboard'" <|
                        setup
                            >> copyTokenButton
                            >> Query.has [ text "copy token to clipboard" ]
                    , test "is 212px wide" <|
                        setup
                            >> copyTokenButton
                            >> Query.has [ style [ ( "width", "212px" ) ] ]
                    , test "has padding above and below" <|
                        setup
                            >> copyTokenButton
                            >> Query.has [ style [ ( "padding", "10px 0" ) ] ]
                    , test "has 10px margin above and below" <|
                        setup
                            >> copyTokenButton
                            >> Query.has [ style [ ( "margin", "10px 0" ) ] ]
                    , test "has centered text" <|
                        setup
                            >> copyTokenButton
                            >> Query.has [ style [ ( "text-align", "center" ) ] ]
                    , test "has pointer cursor" <|
                        setup
                            >> copyTokenButton
                            >> Query.has [ style [ ( "cursor", "pointer" ) ] ]
                    , test "has clipboard attribute that is readable by clipboard.js" <|
                        setup
                            >> copyTokenButton
                            >> Query.has [ attribute <| Attr.attribute "data-clipboard-text" authToken ]
                    , defineHoverBehaviour
                        { name = "copy token button"
                        , setup = setup ()
                        , query = copyTokenButton
                        , updateFunc = \msg -> Layout.update msg >> Tuple.first
                        , unhoveredSelector =
                            { description = "same background as card"
                            , selector = [ style [ ( "background-color", darkGrey ) ] ]
                            }
                        , mouseEnterMsg =
                            Layout.SubMsg 1 <|
                                SubPage.FlySuccessMsg <|
                                    FlySuccess.CopyTokenButtonHover True
                        , mouseLeaveMsg =
                            Layout.SubMsg 1 <|
                                SubPage.FlySuccessMsg <|
                                    FlySuccess.CopyTokenButtonHover False
                        , hoveredSelector =
                            { description = "darker background"
                            , selector = [ style [ ( "background-color", darkerGrey ) ] ]
                            }
                        }
                    , test "clicking sends CopyToken msg" <|
                        setup
                            >> copyTokenButton
                            >> Event.simulate Event.click
                            >> Event.expect
                                (Layout.SubMsg 1 <|
                                    SubPage.FlySuccessMsg <|
                                        FlySuccess.CopyToken
                                )
                    , describe "receiving CopyToken msg" <|
                        let
                            tokenCopied : () -> ( Layout.Model, Cmd Layout.Msg )
                            tokenCopied =
                                setup
                                    >> Layout.update
                                        (Layout.SubMsg 1 <|
                                            SubPage.FlySuccessMsg <|
                                                FlySuccess.CopyToken
                                        )
                        in
                            [ test "changes button background and border to blue" <|
                                tokenCopied
                                    >> Tuple.first
                                    >> copyTokenButton
                                    >> Query.has
                                        [ style
                                            [ ( "background-color", blue )
                                            , ( "border", "1px solid " ++ blue )
                                            ]
                                        ]
                            , test "changes button text to say 'token copied'" <|
                                tokenCopied
                                    >> Tuple.first
                                    >> copyTokenButton
                                    >> Expect.all
                                        [ Query.has
                                            [ text "token copied"
                                            ]
                                        , Query.hasNot
                                            [ text "copy token to clipboard"
                                            ]
                                        ]
                            , test "changes button cursor to default" <|
                                tokenCopied
                                    >> Tuple.first
                                    >> copyTokenButton
                                    >> Query.has
                                        [ style
                                            [ ( "cursor", "default" )
                                            ]
                                        ]
                            ]
                    ]
            ]
