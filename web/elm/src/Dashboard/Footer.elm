module Dashboard.Footer exposing (Model, handleDelivery, view)

import Concourse.Cli as Cli
import Concourse.PipelineStatus as PipelineStatus exposing (PipelineStatus(..))
import Dashboard.Group exposing (Group)
import Dashboard.Msgs exposing (Msg(..))
import Dashboard.Styles as Styles
import Effects
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, href, id, style)
import Html.Events exposing (onMouseEnter, onMouseLeave)
import Routes
import ScreenSize
import Subscription exposing (Delivery(..), Interval(..))
import TopBar.Model exposing (Dropdown(..))


type alias Model r =
    { r
        | hideFooter : Bool
        , hideFooterCounter : Int
        , showHelp : Bool
        , groups : List Group
        , hoveredCliIcon : Maybe Cli.Cli
        , screenSize : ScreenSize.ScreenSize
        , version : String
        , route : Routes.Route
        , shiftDown : Bool
        , dropdown : Dropdown
    }


handleDelivery :
    Delivery
    -> ( Model r, List Effects.Effect )
    -> ( Model r, List Effects.Effect )
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


view : Model r -> Html Msg
view model =
    if model.showHelp then
        keyboardHelp

    else if not model.hideFooter then
        infoBar model

    else
        Html.text ""


keyboardHelp : Html Msg
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
        | hoveredCliIcon : Maybe Cli.Cli
        , screenSize : ScreenSize.ScreenSize
        , version : String
        , route : Routes.Route
        , groups : List Group
    }
    -> Html Msg
infoBar model =
    Html.div
        [ id "dashboard-info"
        , style <|
            Styles.infoBar
                { hideLegend = hideLegend model
                , screenSize = model.screenSize
                }
        ]
        [ legend model
        , concourseInfo model
        ]


legend :
    { a
        | groups : List Group
        , screenSize : ScreenSize.ScreenSize
        , route : Routes.Route
    }
    -> Html Msg
legend model =
    if hideLegend model then
        Html.text ""

    else
        Html.div
            [ id "legend"
            , style Styles.legend
            ]
        <|
            List.map legendItem
                [ PipelineStatusPending False
                , PipelineStatusPaused
                ]
                ++ [ Html.div [ style Styles.legendItem ]
                        [ Html.div [ style Styles.runningLegendItem ] []
                        , Html.div [ style [ ( "width", "10px" ) ] ] []
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
                ++ [ toggleView (model.route == Routes.Dashboard Routes.HighDensity) ]


concourseInfo :
    { a | version : String, hoveredCliIcon : Maybe Cli.Cli }
    -> Html Msg
concourseInfo { version, hoveredCliIcon } =
    Html.div [ id "concourse-info", style Styles.info ]
        [ Html.div [ style Styles.infoItem ]
            [ Html.text <| "version: v" ++ version ]
        , Html.div [ style Styles.infoItem ] <|
            [ Html.span
                [ style [ ( "margin-right", "10px" ) ] ]
                [ Html.text "cli: " ]
            ]
                ++ List.map (cliIcon hoveredCliIcon) Cli.clis
        ]


hideLegend : { a | groups : List Group } -> Bool
hideLegend { groups } =
    List.isEmpty (groups |> List.concatMap .pipelines)


legendItem : PipelineStatus -> Html Msg
legendItem status =
    Html.div [ style Styles.legendItem ]
        [ Html.div
            [ style <| Styles.pipelineStatusIcon status ]
            []
        , Html.div [ style [ ( "width", "10px" ) ] ] []
        , Html.text <| PipelineStatus.show status
        ]


toggleView : Bool -> Html Msg
toggleView highDensity =
    Html.a
        [ style Styles.highDensityToggle
        , href <| Routes.toString <| Routes.dashboardRoute (not highDensity)
        , attribute "aria-label" "Toggle high-density view"
        ]
        [ Html.div [ style <| Styles.highDensityIcon highDensity ] []
        , Html.text "high-density"
        ]


legendSeparator : ScreenSize.ScreenSize -> List (Html Msg)
legendSeparator screenSize =
    case screenSize of
        ScreenSize.Mobile ->
            []

        ScreenSize.Desktop ->
            [ Html.div
                [ style Styles.legendSeparator ]
                [ Html.text "|" ]
            ]

        ScreenSize.BigDesktop ->
            [ Html.div
                [ style Styles.legendSeparator ]
                [ Html.text "|" ]
            ]


cliIcon : Maybe Cli.Cli -> Cli.Cli -> Html Msg
cliIcon hoveredCliIcon cli =
    Html.a
        [ href (Cli.downloadUrl cli)
        , attribute "aria-label" <| Cli.label cli
        , style <|
            Styles.infoCliIcon
                { hovered = hoveredCliIcon == Just cli
                , cli = cli
                }
        , id <| "cli-" ++ Cli.id cli
        , onMouseEnter <| CliHover <| Just cli
        , onMouseLeave <| CliHover Nothing
        ]
        []
