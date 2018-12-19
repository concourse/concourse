module Dashboard.Footer exposing (Model, showFooter, tick, toggleHelp, view)

import Concourse.Cli as Cli
import Concourse.PipelineStatus as PipelineStatus exposing (PipelineStatus(..))
import Dashboard.Group exposing (Group)
import Dashboard.Msgs exposing (Msg(..))
import Dashboard.Styles as Styles
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, href, id, style)
import Html.Events exposing (onMouseEnter, onMouseLeave)
import Routes
import ScreenSize


type alias Model r =
    { r
        | hideFooter : Bool
        , hideFooterCounter : Int
        , showHelp : Bool
        , groups : List Group
        , hoveredCliIcon : Maybe Cli.Cli
        , screenSize : ScreenSize.ScreenSize
        , version : String
        , highDensity : Bool
    }


showFooter : Model r -> Model r
showFooter model =
    { model | hideFooter = False, hideFooterCounter = 0 }


tick : Model r -> Model r
tick model =
    if model.hideFooterCounter > 4 then
        { model | hideFooter = True }
    else
        { model | hideFooterCounter = model.hideFooterCounter + 1 }


toggleHelp : Model r -> Model r
toggleHelp model =
    { model | showHelp = not (hideHelp model || model.showHelp) }


hideHelp : { a | groups : List Group } -> Bool
hideHelp { groups } =
    List.isEmpty (groups |> List.concatMap .pipelines)


view : Model r -> List (Html Msg)
view model =
    if model.showHelp then
        [ keyboardHelp ]
    else if not model.hideFooter then
        [ infoBar model ]
    else
        []


keyboardHelp : Html Msg
keyboardHelp =
    Html.div
        [ class "keyboard-help" ]
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
        , highDensity : Bool
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
    <|
        legend model
            ++ concourseInfo model


legend :
    { a
        | groups : List Group
        , screenSize : ScreenSize.ScreenSize
        , highDensity : Bool
    }
    -> List (Html Msg)
legend model =
    if hideLegend model then
        []
    else
        [ Html.div
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
                ++ [ toggleView model.highDensity ]
        ]


concourseInfo :
    { a | version : String, hoveredCliIcon : Maybe Cli.Cli }
    -> List (Html Msg)
concourseInfo { version, hoveredCliIcon } =
    [ Html.div [ id "concourse-info", style Styles.info ]
        [ Html.div [ style Styles.infoItem ]
            [ Html.text <| "version: v" ++ version ]
        , Html.div [ style Styles.infoItem ]
            [ Html.span
                [ style [ ( "margin-right", "10px" ) ] ]
                [ Html.text "cli: " ]
            , cliIcon Cli.OSX hoveredCliIcon
            , cliIcon Cli.Windows hoveredCliIcon
            , cliIcon Cli.Linux hoveredCliIcon
            ]
        ]
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
    let
        route =
            if highDensity then
                Routes.dashboardRoute
            else
                Routes.dashboardHdRoute
    in
        Html.a
            [ style Styles.highDensityToggle
            , href route
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


cliIcon : Cli.Cli -> Maybe Cli.Cli -> Html Msg
cliIcon cli hoveredCliIcon =
    let
        ( cliName, ariaText, icon ) =
            case cli of
                Cli.OSX ->
                    ( "osx", "OS X", "apple" )

                Cli.Windows ->
                    ( "windows", "Windows", "windows" )

                Cli.Linux ->
                    ( "linux", "Linux", "linux" )
    in
        Html.a
            [ href (Cli.downloadUrl "amd64" cliName)
            , attribute "aria-label" <| "Download " ++ ariaText ++ " CLI"
            , style <| Styles.infoCliIcon (hoveredCliIcon == Just cli)
            , id <| "cli-" ++ cliName
            , onMouseEnter <| CliHover <| Just cli
            , onMouseLeave <| CliHover Nothing
            ]
            [ Html.i [ class <| "fa fa-" ++ icon ] [] ]
