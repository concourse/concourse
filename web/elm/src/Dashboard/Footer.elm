module Dashboard.Footer exposing (handleDelivery, view)

import Concourse.Cli as Cli
import Concourse.PipelineStatus as PipelineStatus exposing (PipelineStatus(..))
import Dashboard.Group.Models exposing (Group)
import Dashboard.Models exposing (Dropdown(..), FooterModel)
import Dashboard.Styles as Styles
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, download, href, id, style)
import Html.Events exposing (onMouseEnter, onMouseLeave)
import Message.Effects as Effects
import Message.Message exposing (Hoverable(..), Message(..))
import Message.Subscription exposing (Delivery(..), Interval(..))
import Routes
import ScreenSize
import Views.Icon as Icon


handleDelivery :
    Delivery
    -> ( FooterModel r, List Effects.Effect )
    -> ( FooterModel r, List Effects.Effect )
handleDelivery delivery ( model, effects ) =
    case delivery of
        KeyDown keyCode ->
            case keyCode of
                -- '/' key
                191 ->
                    if model.shiftDown && model.dropdown == Hidden then
                        ( { model
                            | showHelp =
                                if
                                    model.groups
                                        |> List.concatMap .pipelines
                                        |> List.isEmpty
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

        Moused ->
            ( { model | hideFooter = False, hideFooterCounter = 0 }, effects )

        ClockTicked OneSecond time ->
            ( if model.hideFooterCounter > 4 then
                { model | hideFooter = True }

              else
                { model | hideFooterCounter = model.hideFooterCounter + 1 }
            , effects
            )

        _ ->
            ( model, effects )


view : FooterModel r -> Html Message
view model =
    if model.showHelp then
        keyboardHelp

    else if not model.hideFooter then
        infoBar model

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
        | hovered : Maybe Hoverable
        , screenSize : ScreenSize.ScreenSize
        , version : String
        , highDensity : Bool
        , groups : List Group
    }
    -> Html Message
infoBar model =
    Html.div
        ([ id "dashboard-info" ]
            ++ Styles.infoBar
                { hideLegend = hideLegend model
                , screenSize = model.screenSize
                }
        )
        [ legend model
        , concourseInfo model
        ]


legend :
    { a
        | groups : List Group
        , screenSize : ScreenSize.ScreenSize
        , highDensity : Bool
    }
    -> Html Message
legend model =
    if hideLegend model then
        Html.text ""

    else
        Html.div
            ([ id "legend" ] ++ Styles.legend)
        <|
            List.map legendItem
                [ PipelineStatusPending False
                , PipelineStatusPaused
                ]
                ++ [ Html.div
                        Styles.legendItem
                        [ Icon.icon
                            { sizePx = 20
                            , image = "ic-running-legend.svg"
                            }
                            []
                        , Html.div [ style "width" "10px" ] []
                        , Html.text "running"
                        ]
                   ]
                ++ List.map legendItem
                    [ PipelineStatusFailed PipelineStatus.Running
                    , PipelineStatusErrored PipelineStatus.Running
                    , PipelineStatusAborted PipelineStatus.Running
                    , PipelineStatusSucceeded PipelineStatus.Running
                    ]
                ++ legendSeparator model.screenSize
                ++ [ toggleView model.highDensity ]


concourseInfo :
    { a | version : String, hovered : Maybe Hoverable }
    -> Html Message
concourseInfo { version, hovered } =
    Html.div ([ id "concourse-info" ] ++ Styles.info)
        [ Html.div
            Styles.infoItem
            [ Html.text <| "version: v" ++ version ]
        , Html.div Styles.infoItem <|
            [ Html.span
                [ style "margin-right" "10px" ]
                [ Html.text "cli: " ]
            ]
                ++ List.map (cliIcon hovered) Cli.clis
        ]


hideLegend : { a | groups : List Group } -> Bool
hideLegend { groups } =
    List.isEmpty (groups |> List.concatMap .pipelines)


legendItem : PipelineStatus -> Html Message
legendItem status =
    Html.div
        Styles.legendItem
        [ PipelineStatus.icon status
        , Html.div [ style "width" "10px" ] []
        , Html.text <| PipelineStatus.show status
        ]


toggleView : Bool -> Html Message
toggleView highDensity =
    Html.a
        ([ href <| Routes.toString <| Routes.dashboardRoute (not highDensity)
         , attribute "aria-label" "Toggle high-density view"
         ]
            ++ Styles.highDensityToggle
        )
        [ Html.div (Styles.highDensityIcon highDensity) []
        , Html.text "high-density"
        ]


legendSeparator : ScreenSize.ScreenSize -> List (Html Message)
legendSeparator screenSize =
    case screenSize of
        ScreenSize.Mobile ->
            []

        ScreenSize.Desktop ->
            [ Html.div Styles.legendSeparator [ Html.text "|" ] ]

        ScreenSize.BigDesktop ->
            [ Html.div Styles.legendSeparator [ Html.text "|" ] ]


cliIcon : Maybe Hoverable -> Cli.Cli -> Html Message
cliIcon hovered cli =
    Html.a
        ([ href <| Cli.downloadUrl cli
         , attribute "aria-label" <| Cli.label cli
         , id <| "cli-" ++ Cli.id cli
         , onMouseEnter <| Hover <| Just <| FooterCliIcon cli
         , onMouseLeave <| Hover Nothing
         , download ""
         ]
            ++ Styles.infoCliIcon
                { hovered = hovered == (Just <| FooterCliIcon cli)
                , cli = cli
                }
        )
        []
