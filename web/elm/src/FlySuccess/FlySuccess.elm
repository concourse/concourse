module FlySuccess.FlySuccess exposing
    ( documentTitle
    , handleDelivery
    , init
    , subscriptions
    , update
    , view
    )

import EffectTransformer exposing (ET)
import FlySuccess.Models as Models exposing (ButtonState(..), Model, hover)
import FlySuccess.Styles as Styles
import FlySuccess.Text as Text
import Html exposing (Html)
import Html.Attributes exposing (attribute, href, id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Login.Login as Login
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription as Subscription
    exposing
        ( Delivery(..)
        , Subscription(..)
        )
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Routes
import UserState exposing (UserState)
import Views.Icon as Icon
import Views.Styles
import Views.TopBar as TopBar


init :
    { authToken : String
    , flyPort : Maybe Int
    , noop : Bool
    }
    -> ( Model, List Effect )
init { authToken, flyPort, noop } =
    ( { buttonState = Unhovered
      , authToken = authToken
      , tokenTransfer =
            case ( noop, flyPort ) of
                ( False, Just _ ) ->
                    Models.Pending

                ( False, Nothing ) ->
                    Models.NoFlyPort

                ( True, _ ) ->
                    Models.Success
      , isUserMenuExpanded = False
      , flyPort = flyPort
      }
    , case ( noop, flyPort ) of
        ( False, Just fp ) ->
            [ SendTokenToFly authToken fp ]

        _ ->
            []
    )


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        TokenSentToFly Subscription.Success ->
            ( { model | tokenTransfer = Models.Success }, effects )

        TokenSentToFly Subscription.NetworkError ->
            ( { model | tokenTransfer = Models.NetworkTrouble }
            , effects
            )

        TokenSentToFly Subscription.BrowserError ->
            ( { model | tokenTransfer = Models.BlockedByBrowser }
            , effects
            )

        _ ->
            ( model, effects )


update : Message -> ET Model
update msg ( model, effects ) =
    case msg of
        Hover (Just CopyTokenButton) ->
            ( { model | buttonState = hover True model.buttonState }
            , effects
            )

        Hover Nothing ->
            ( { model | buttonState = hover False model.buttonState }
            , effects
            )

        Click CopyTokenButton ->
            ( { model | buttonState = Clicked }, effects )

        _ ->
            ( model, effects )


subscriptions : List Subscription
subscriptions =
    [ OnTokenSentToFly ]


documentTitle : String
documentTitle =
    "Fly Login"


view : UserState -> Model -> Html Message
view userState model =
    Html.div []
        [ Html.div
            (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
            [ Html.div
                (id "top-bar-app" :: Views.Styles.topBar False)
                [ TopBar.concourseLogo
                , Login.view userState model False
                ]
            , Html.div
                (id "page-below-top-bar"
                    :: (Views.Styles.pageBelowTopBar <|
                            Routes.FlySuccess False Nothing
                       )
                )
                [ Html.div
                    (id "success-card" :: Styles.card)
                    [ Html.p
                        (id "success-card-title" :: Styles.title)
                        [ Html.text Text.title ]
                    , Html.div
                        (id "success-card-body" :: Styles.body)
                      <|
                        body model
                    ]
                ]
            ]
        ]


body : Model -> List (Html Message)
body model =
    let
        elemList =
            List.filter Tuple.second >> List.map Tuple.first
    in
    case model.tokenTransfer of
        Models.Pending ->
            [ Html.text Text.pending ]

        Models.Success ->
            elemList
                [ ( paragraph
                        { identifier = "first-paragraph"
                        , lines = Text.firstParagraphSuccess
                        }
                  , True
                  )
                , ( button model, False )
                , ( paragraph
                        { identifier = "second-paragraph"
                        , lines = Text.secondParagraphSuccess
                        }
                  , True
                  )
                ]

        Models.BlockedByBrowser ->
            elemList
                [ ( paragraph
                        { identifier = "first-paragraph"
                        , lines = Text.firstParagraphFailure
                        }
                  , True
                  )
                , ( button model, False )
                , ( paragraph
                        { identifier = "second-paragraph"
                        , lines = Text.secondParagraphFailure Models.BlockedByBrowser
                        }
                  , True
                  )
                , ( flyLoginLink model, True )
                , ( paragraph
                        { identifier = "third-paragraph"
                        , lines = Text.thirdParagraphBlocked
                        }
                  , True
                  )
                , ( button model, True )
                , ( paragraph
                        { identifier = "fourth-paragraph"
                        , lines = Text.secondParagraphFailure Models.NetworkTrouble
                        }
                  , True
                  )
                ]

        err ->
            elemList
                [ ( paragraph
                        { identifier = "first-paragraph"
                        , lines = Text.firstParagraphFailure
                        }
                  , True
                  )
                , ( button model, True )
                , ( paragraph
                        { identifier = "second-paragraph"
                        , lines = Text.secondParagraphFailure err
                        }
                  , True
                  )
                ]


paragraph : { identifier : String, lines : Text.Paragraph } -> Html Message
paragraph { identifier, lines } =
    lines
        |> List.map Html.text
        |> List.intersperse (Html.br [] [])
        |> Html.p (id identifier :: Styles.paragraph)


button : Model -> Html Message
button { authToken, buttonState } =
    Html.span
        ([ id "copy-token"
         , onMouseEnter <| Hover <| Just CopyTokenButton
         , onMouseLeave <| Hover Nothing
         , onClick <| Click CopyTokenButton
         , attribute "data-clipboard-text" authToken
         ]
            ++ Styles.button buttonState
        )
        [ Icon.icon
            { sizePx = 20
            , image = "clippy.svg"
            }
            [ id "copy-icon"
            , style "margin-right" "5px"
            ]
        , Html.text <| Text.button buttonState
        ]


flyLoginLink : Model -> Html Message
flyLoginLink { flyPort, authToken } =
    case flyPort of
        Just fp ->
            Html.a
                [ href (Routes.tokenToFlyRoute authToken fp)
                , id "link"
                , style "text-decoration" "underline"
                , style "line-height" "2"
                ]
                [ Html.text Text.flyLoginLinkText ]

        Nothing ->
            Html.text ""
