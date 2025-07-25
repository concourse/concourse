module Dashboard.Footer exposing (handleDelivery, view)

import Assets
import Concourse.PipelineStatus as PipelineStatus exposing (PipelineStatus(..))
import Dashboard.Filter as Filter
import Dashboard.Group.Models exposing (Pipeline)
import Dashboard.Models exposing (Dropdown(..), FooterModel)
import Dashboard.Styles as Styles
import Dict exposing (Dict)
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, download, href, id, rel, src, style, target, title)
import Html.Events exposing (onMouseEnter, onMouseLeave)
import Keyboard
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..), Interval(..))
import Routes
import ScreenSize
import Views.Icon as Icon
import Views.Toggle as Toggle


handleDelivery :
    Delivery
    -> ( FooterModel r, List Effects.Effect )
    -> ( FooterModel r, List Effects.Effect )
handleDelivery delivery ( model, effects ) =
    case delivery of
        KeyDown keyEvent ->
            case keyEvent.code of
                -- '/' key
                Keyboard.Slash ->
                    if keyEvent.shiftKey && model.dropdown == Hidden then
                        ( { model
                            | showHelp =
                                if
                                    model.pipelines
                                        |> Maybe.withDefault Dict.empty
                                        |> Dict.values
                                        |> List.all List.isEmpty
                                then
                                    False

                                else
                                    not model.showHelp
                          }
                        , effects
                        )

                    else
                        ( model, effects )

                _ ->
                    ( { model | hideFooter = False, hideFooterCounter = 0 }
                    , effects
                    )

        Moused _ ->
            ( { model | hideFooter = False, hideFooterCounter = 0 }, effects )

        ClockTicked OneSecond _ ->
            ( if model.hideFooterCounter > 8 then
                { model | hideFooter = True }

              else
                { model | hideFooterCounter = model.hideFooterCounter + 1 }
            , effects
            )

        _ ->
            ( model, effects )


view :
    { a
        | hovered : HoverState.HoverState
        , screenSize : ScreenSize.ScreenSize
        , version : String
    }
    -> FooterModel r
    -> Html Message
view session model =
    if model.showHelp then
        keyboardHelp

    else if not model.hideFooter then
        infoBar session model

    else
        Html.text ""


keyboardHelp : Html Message
keyboardHelp =
    Html.div
        [ class "keyboard-help", id "keyboard-help" ]
        [ Html.div
            [ class "help-title" ]
            [ Html.text "keyboard shortcuts" ]
        , Html.div
            [ class "help-line" ]
            [ Html.div
                [ class "keys" ]
                [ Html.span
                    [ class "key" ]
                    [ Html.text "/" ]
                ]
            , Html.text "search"
            ]
        , Html.div
            [ class "help-line" ]
            [ Html.div
                [ class "keys" ]
                [ Html.span
                    [ class "key" ]
                    [ Html.text "?" ]
                ]
            , Html.text "hide/show help"
            ]
        ]


infoBar :
    { a
        | hovered : HoverState.HoverState
        , screenSize : ScreenSize.ScreenSize
        , version : String
    }
    -> FooterModel r
    -> Html Message
infoBar session model =
    Html.div
        (id "dashboard-info"
            :: Styles.infoBar
                { hideLegend = hideLegend model
                , screenSize = session.screenSize
                }
        )
        [ legend session model
        , concourseInfo session
        ]


legend :
    { a | screenSize : ScreenSize.ScreenSize }
    -> FooterModel r
    -> Html Message
legend session model =
    if hideLegend model then
        Html.text ""

    else
        Html.div
            (id "legend" :: Styles.legend)
        <|
            List.map legendItem
                [ PipelineStatusPending False
                , PipelineStatusPaused
                ]
                ++ Html.div
                    Styles.legendItem
                    [ Icon.icon
                        { sizePx = 20
                        , image = Assets.RunningLegend
                        }
                        []
                    , Html.div [ style "width" "10px" ] []
                    , Html.text "running"
                    ]
                :: List.map legendItem
                    [ PipelineStatusFailed PipelineStatus.Running
                    , PipelineStatusErrored PipelineStatus.Running
                    , PipelineStatusAborted PipelineStatus.Running
                    , PipelineStatusSucceeded PipelineStatus.Running
                    ]
                ++ (if Filter.isViewingInstanceGroups model.query then
                        []

                    else
                        legendSeparator session.screenSize
                            ++ [ toggleView model ]
                   )


concourseInfo :
    { a | hovered : HoverState.HoverState, version : String }
    -> Html Message
concourseInfo { hovered, version } =
    Html.div (id "concourse-info" :: Styles.info)
        [ Html.div
            Styles.footerLink
            [ Html.a
                [ href "https://concourse-ci.org/docs.html"
                , target "_blank"
                , rel "noopener noreferrer"
                , style "align-items" "center"
                , style "display" "flex"
                ]
                [ Html.div Styles.docsIcon []
                , Html.text "Docs"
                ]
            ]
        , Html.div Styles.footerLink
            [ Html.a
                [ href "/download-fly"
                , style "align-items" "center"
                , style "display" "flex"
                ]
                [ Html.div Styles.consoleIcon []
                , Html.text "Download fly cli"
                ]
            ]
        , Html.div
            [ id "version-info" ]
            [ Html.text <| "Version: v" ++ version ]
        ]


hideLegend : { a | pipelines : Maybe (Dict String (List Pipeline)) } -> Bool
hideLegend { pipelines } =
    pipelines
        |> Maybe.withDefault Dict.empty
        |> Dict.values
        |> List.all List.isEmpty


legendItem : PipelineStatus -> Html Message
legendItem status =
    Html.div
        Styles.legendItem
        [ case Assets.pipelineStatusIcon status of
            Just asset ->
                Icon.icon
                    { sizePx = 20, image = asset }
                    Styles.pipelineStatusIcon

            Nothing ->
                Html.text ""
        , Html.div [ style "width" "10px" ] []
        , Html.text <| PipelineStatus.show status
        ]


toggleView : FooterModel r -> Html Message
toggleView { highDensity, dashboardView } =
    Toggle.toggleSwitch
        { ariaLabel = "Toggle high-density view"
        , hrefRoute =
            Routes.Dashboard
                { searchType =
                    if highDensity then
                        Routes.Normal ""

                    else
                        Routes.HighDensity
                , dashboardView = dashboardView
                }
        , text = "high-density"
        , textDirection = Toggle.Right
        , on = highDensity
        , styles = Styles.highDensityToggle
        }


legendSeparator : ScreenSize.ScreenSize -> List (Html Message)
legendSeparator screenSize =
    case screenSize of
        ScreenSize.Mobile ->
            []

        ScreenSize.Desktop ->
            [ Html.div Styles.legendSeparator [ Html.text "|" ] ]

        ScreenSize.BigDesktop ->
            [ Html.div Styles.legendSeparator [ Html.text "|" ] ]
