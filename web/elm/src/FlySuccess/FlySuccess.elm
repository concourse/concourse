module FlySuccess.FlySuccess exposing
    ( documentTitle
    , handleDelivery
    , init
    , subscriptions
    , tooltip
    , update
    , view
    )

import Assets
import EffectTransformer exposing (ET)
import FlySuccess.Models as Models exposing (ButtonState(..), InputState(..), Model, hover)
import FlySuccess.Styles as Styles
import FlySuccess.Text as Text
import Html exposing (Html)
import Html.Attributes exposing (attribute, href, id, style, value)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Login.Login as Login
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription as Subscription
    exposing
        ( Delivery(..)
        , RawHttpResponse(..)
        , Subscription(..)
        )
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Routes
import Tooltip
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
    ( { copyTokenButtonState = Unhovered
      , sendTokenButtonState = Unhovered
      , copyTokenInputState = InputUnhovered
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
            ( { model | tokenTransfer = Models.NetworkTrouble }, effects )

        TokenSentToFly Subscription.BrowserError ->
            ( { model | tokenTransfer = Models.BlockedByBrowser }, effects )

        _ ->
            ( model, effects )


update : Message -> ET Model
update msg ( model, effects ) =
    case msg of
        Hover (Just CopyTokenButton) ->
            ( { model
                | copyTokenButtonState = hover True model.copyTokenButtonState
              }
            , effects
            )

        Hover (Just SendTokenButton) ->
            ( { model
                | sendTokenButtonState = hover True model.sendTokenButtonState
              }
            , effects
            )

        Hover (Just CopyTokenInput) ->
            ( { model | copyTokenInputState = InputHovered }, effects )

        Hover Nothing ->
            ( { model
                | copyTokenButtonState = hover False model.copyTokenButtonState
                , sendTokenButtonState = hover False model.sendTokenButtonState
                , copyTokenInputState = InputUnhovered
              }
            , effects
            )

        Click CopyTokenButton ->
            ( { model | copyTokenButtonState = Clicked }, effects )

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


tooltip : Model -> a -> Maybe Tooltip.Tooltip
tooltip _ _ =
    Nothing


body : Model -> List (Html Message)
body model =
    let
        p1 =
            paragraph
                { identifier = "first-paragraph"
                , lines = Text.firstParagraph model.tokenTransfer
                }

        p2 =
            paragraph
                { identifier = "second-paragraph"
                , lines = Text.secondParagraph model.tokenTransfer
                }
    in
    case model.tokenTransfer of
        Models.Pending ->
            [ Html.text Text.pending ]

        Models.Success ->
            [ p1, p2 ]

        Models.NetworkTrouble ->
            [ p1, tokenTextBox model, copyTokenButton model, p2 ]

        Models.BlockedByBrowser ->
            [ p1, tokenTextBox model, sendTokenButton model, p2, copyTokenButton model ]

        Models.NoFlyPort ->
            [ p1, tokenTextBox model, copyTokenButton model, p2 ]


tokenTextBox : Model -> Html Message
tokenTextBox { copyTokenInputState, authToken } =
    Html.label []
        [ Html.text Text.copyTokenInput
        , Html.input
            ([ id "manual-copy-token"
             , value authToken
             , onMouseEnter <| Hover <| Just CopyTokenInput
             , onMouseLeave <| Hover Nothing
             ]
                ++ Styles.input copyTokenInputState
            )
            []
        ]


paragraph : { identifier : String, lines : Text.Paragraph } -> Html Message
paragraph { identifier, lines } =
    lines
        |> List.map Html.text
        |> List.intersperse (Html.br [] [])
        |> Html.p (id identifier :: Styles.paragraph)


copyTokenButton : Model -> Html Message
copyTokenButton { authToken, copyTokenButtonState } =
    Html.span
        ([ id "copy-token"
         , onMouseEnter <| Hover <| Just CopyTokenButton
         , onMouseLeave <| Hover Nothing
         , onClick <| Click CopyTokenButton
         , attribute "data-clipboard-text" authToken
         ]
            ++ Styles.button copyTokenButtonState
        )
        [ Icon.icon
            { sizePx = 20
            , image = Assets.ClippyIcon
            }
            [ id "copy-icon"
            , style "margin-right" "5px"
            ]
        , Html.text <| Text.copyTokenButton copyTokenButtonState
        ]


sendTokenButton : Model -> Html Message
sendTokenButton { sendTokenButtonState, flyPort, authToken } =
    Html.a
        ([ id "send-token"
         , onMouseEnter <| Hover <| Just SendTokenButton
         , onMouseLeave <| Hover Nothing
         , href
            (Maybe.map (Routes.tokenToFlyRoute authToken) flyPort
                |> Maybe.withDefault ""
            )
         ]
            ++ Styles.button sendTokenButtonState
        )
        [ Html.text <| Text.sendTokenButton ]
