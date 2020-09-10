module Dashboard.Group exposing
    ( PipelineIndex
    , hdView
    , ordering
    , pipelineNotSetView
    , view
    , viewFavoritePipelines
    )

import Application.Models exposing (Session)
import Concourse
import Dashboard.Grid as Grid
import Dashboard.Grid.Constants as GridConstants
import Dashboard.Group.Models exposing (Group, Pipeline)
import Dashboard.Group.Tag as Tag
import Dashboard.Models exposing (DragState(..), DropState(..))
import Dashboard.Pipeline as Pipeline
import Dashboard.Styles as Styles
import Dict exposing (Dict)
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, draggable, id, style)
import Html.Events exposing (on, onMouseOut, onMouseOver, preventDefaultOn, stopPropagationOn)
import Html.Keyed
import Json.Decode
import Maybe.Extra
import Message.Effects as Effects
import Message.Message exposing (DomID(..), DropTarget(..), Message(..), PipelinesSection(..))
import Ordering exposing (Ordering)
import Set exposing (Set)
import Time
import UserState exposing (UserState(..))
import Views.Spinner as Spinner
import Views.Styles


ordering : { a | userState : UserState } -> Ordering Group
ordering session =
    Ordering.byFieldWith Tag.ordering (tag session)
        |> Ordering.breakTiesWith (Ordering.byField .teamName)


type alias PipelineIndex =
    Int


view :
    Session
    ->
        { dragState : DragState
        , dropState : DropState
        , now : Maybe Time.Posix
        , pipelinesWithResourceErrors : Set Concourse.DatabaseID
        , pipelineLayers : Dict Concourse.DatabaseID (List (List Concourse.JobIdentifier))
        , pipelineCards : List Grid.Card
        , dropAreas : List Grid.DropArea
        , groupCardsHeight : Float
        , pipelineJobs : Dict Concourse.DatabaseID (List Concourse.JobIdentifier)
        , jobs : Dict ( Concourse.DatabaseID, String ) Concourse.Job
        }
    -> Group
    -> Html Message
view session params g =
    let
        pipelineCardViews =
            if List.isEmpty params.pipelineCards then
                [ ( "not-set", Pipeline.pipelineNotSetView ) ]

            else
                params.pipelineCards
                    |> List.map
                        (\{ bounds, pipeline } ->
                            pipelineCardView session
                                params
                                AllPipelinesSection
                                { bounds = bounds, pipeline = pipeline }
                                g.teamName
                                |> (\html -> ( String.fromInt pipeline.id, html ))
                        )

        dropAreaViews =
            params.dropAreas
                |> List.map
                    (\{ bounds, target } ->
                        pipelineDropAreaView params.dragState g.teamName bounds target
                    )
    in
    Html.div
        [ id <| Effects.toHtmlID <| DashboardGroup g.teamName
        , class "dashboard-team-group"
        , attribute "data-team-name" g.teamName
        ]
        [ Html.div
            [ style "display" "flex"
            , style "align-items" "center"
            , style "margin-bottom" (String.fromInt GridConstants.padding ++ "px")
            , class <| .sectionHeaderClass Effects.stickyHeaderConfig
            ]
            (Html.div
                [ class "dashboard-team-name"
                , style "font-weight" Views.Styles.fontWeightBold
                ]
                [ Html.text g.teamName ]
                :: (Maybe.Extra.toList <|
                        Maybe.map (Tag.view False) (tag session g)
                   )
                ++ (if params.dropState == DroppingWhileApiRequestInFlight g.teamName then
                        [ Spinner.spinner { sizePx = 20, margin = "0 0 0 10px" } ]

                    else
                        []
                   )
            )
        , Html.Keyed.node "div"
            [ class <| .sectionBodyClass Effects.stickyHeaderConfig
            , style "position" "relative"
            , style "height" <| String.fromFloat params.groupCardsHeight ++ "px"
            ]
            (pipelineCardViews
                ++ [ ( "drop-areas", Html.div [ style "position" "absolute" ] dropAreaViews ) ]
            )
        ]


viewFavoritePipelines :
    Session
    ->
        { dragState : DragState
        , dropState : DropState
        , now : Maybe Time.Posix
        , pipelinesWithResourceErrors : Set Concourse.DatabaseID
        , pipelineLayers : Dict Concourse.DatabaseID (List (List Concourse.JobIdentifier))
        , pipelineCards : List Grid.Card
        , headers : List Grid.Header
        , groupCardsHeight : Float
        , pipelineJobs : Dict Concourse.DatabaseID (List Concourse.JobIdentifier)
        , jobs : Dict ( Concourse.DatabaseID, String ) Concourse.Job
        }
    -> Html Message
viewFavoritePipelines session params =
    let
        pipelineCardViews =
            params.pipelineCards
                |> List.map
                    (\{ bounds, pipeline } ->
                        pipelineCardView session
                            params
                            FavoritesSection
                            { bounds = bounds, pipeline = pipeline }
                            pipeline.teamName
                            |> (\html -> ( String.fromInt pipeline.id, html ))
                    )

        headerViews =
            params.headers
                |> List.map
                    (\{ bounds, header } ->
                        headerView bounds header
                    )
    in
    Html.Keyed.node "div"
        [ id <| "dashboard-favorite-pipelines"
        , style "position" "relative"
        , style "height" <| String.fromFloat params.groupCardsHeight ++ "px"
        ]
        (pipelineCardViews
            ++ [ ( "headers"
                 , Html.div
                    [ style "position" "absolute"
                    , class "headers"
                    ]
                    headerViews
                 )
               ]
        )


tag : { a | userState : UserState } -> Group -> Maybe Tag.Tag
tag { userState } g =
    case userState of
        UserStateLoggedIn user ->
            Tag.tag user g.teamName

        _ ->
            Nothing


hdView :
    { pipelineRunningKeyframes : String
    , pipelinesWithResourceErrors : Set Concourse.DatabaseID
    , pipelineJobs : Dict Concourse.DatabaseID (List Concourse.JobIdentifier)
    , jobs : Dict ( Concourse.DatabaseID, String ) Concourse.Job
    }
    -> { a | userState : UserState }
    -> Group
    -> List (Html Message)
hdView { pipelineRunningKeyframes, pipelinesWithResourceErrors, pipelineJobs, jobs } session g =
    let
        orderedPipelines =
            g.pipelines

        header =
            Html.div
                [ class "dashboard-team-name" ]
                [ Html.text g.teamName ]
                :: (Maybe.Extra.toList <| Maybe.map (Tag.view True) (tag session g))

        teamPipelines =
            if List.isEmpty orderedPipelines then
                [ pipelineNotSetView ]

            else
                orderedPipelines
                    |> List.map
                        (\p ->
                            Pipeline.hdPipelineView
                                { pipeline = p
                                , pipelineRunningKeyframes = pipelineRunningKeyframes
                                , resourceError =
                                    pipelinesWithResourceErrors
                                        |> Set.member p.id
                                , existingJobs =
                                    pipelineJobs
                                        |> Dict.get p.id
                                        |> Maybe.withDefault []
                                        |> List.filterMap (lookupJob jobs)
                                }
                        )
    in
    case teamPipelines of
        [] ->
            header

        p :: ps ->
            -- Wrap the team name and the first pipeline together so
            -- the team name is not the last element in a column
            Html.div
                (class "dashboard-team-name-wrapper" :: Styles.teamNameHd)
                (header ++ [ p ])
                :: ps


pipelineNotSetView : Html Message
pipelineNotSetView =
    Html.div
        [ class "card" ]
        [ Html.div
            Styles.noPipelineCardHd
            [ Html.div
                Styles.noPipelineCardTextHd
                [ Html.text "no pipelines set" ]
            ]
        ]


lookupJob : Dict ( Concourse.DatabaseID, String ) Concourse.Job -> Concourse.JobIdentifier -> Maybe Concourse.Job
lookupJob jobs jobId =
    jobs
        |> Dict.get ( jobId.pipelineId, jobId.jobName )


pipelineCardView :
    Session
    ->
        { b
            | dragState : DragState
            , dropState : DropState
            , now : Maybe Time.Posix
            , pipelinesWithResourceErrors : Set Concourse.DatabaseID
            , pipelineLayers : Dict Concourse.DatabaseID (List (List Concourse.JobIdentifier))
            , pipelineJobs : Dict Concourse.DatabaseID (List Concourse.JobIdentifier)
            , jobs : Dict ( Concourse.DatabaseID, String ) Concourse.Job
        }
    -> PipelinesSection
    ->
        { bounds : Grid.Bounds
        , pipeline : Pipeline
        }
    -> String
    -> Html Message
pipelineCardView session params section { bounds, pipeline } teamName =
    Html.div
        ([ class "pipeline-wrapper"
         , style "position" "absolute"
         , style "transform"
            ("translate("
                ++ String.fromFloat bounds.x
                ++ "px,"
                ++ String.fromFloat bounds.y
                ++ "px)"
            )
         , style
            "width"
            (String.fromFloat bounds.width
                ++ "px"
            )
         , style "height"
            (String.fromFloat bounds.height
                ++ "px"
            )
         , onMouseOver <|
            Hover <|
                Just <|
                    PipelineWrapper pipeline.id
         , onMouseOut <| Hover Nothing
         ]
            ++ (if params.dragState /= NotDragging then
                    [ style "transition" "transform 0.2s ease-in-out" ]

                else
                    []
               )
            ++ (let
                    hoverStyle id =
                        if id == pipeline.id then
                            [ style "z-index" "1" ]

                        else
                            []
                in
                case HoverState.hoveredElement session.hovered of
                    Just (JobPreview _ jobID) ->
                        hoverStyle jobID.pipelineId

                    Just (PipelineWrapper pipelineId) ->
                        hoverStyle pipelineId

                    _ ->
                        []
               )
        )
        [ Html.div
            ([ class "card"
             , style "width" "100%"
             , style "height" "100%"
             , attribute "data-pipeline-name" pipeline.name
             ]
                ++ (if section == AllPipelinesSection && not pipeline.stale then
                        [ attribute
                            "ondragstart"
                            "event.dataTransfer.setData('text/plain', '');"
                        , draggable "true"
                        , on "dragstart"
                            (Json.Decode.succeed (DragStart pipeline.teamName pipeline.name))
                        , on "dragend" (Json.Decode.succeed DragEnd)
                        ]

                    else
                        []
                   )
                ++ (if params.dragState == Dragging pipeline.teamName pipeline.name then
                        [ style "width" "0"
                        , style "margin" "0 12.5px"
                        , style "overflow" "hidden"
                        ]

                    else
                        []
                   )
                ++ (if params.dropState == DroppingWhileApiRequestInFlight teamName then
                        [ style "opacity" "0.45", style "pointer-events" "none" ]

                    else
                        [ style "opacity" "1" ]
                   )
            )
            [ Pipeline.pipelineView session
                { now = params.now
                , pipeline = pipeline
                , resourceError =
                    params.pipelinesWithResourceErrors
                        |> Set.member pipeline.id
                , existingJobs =
                    params.pipelineJobs
                        |> Dict.get pipeline.id
                        |> Maybe.withDefault []
                        |> List.filterMap (lookupJob params.jobs)
                , layers =
                    params.pipelineLayers
                        |> Dict.get pipeline.id
                        |> Maybe.withDefault []
                        |> List.map (List.filterMap <| lookupJob params.jobs)
                , hovered = session.hovered
                , pipelineRunningKeyframes = session.pipelineRunningKeyframes
                , section = section
                }
            ]
        ]


pipelineDropAreaView : DragState -> String -> Grid.Bounds -> DropTarget -> Html Message
pipelineDropAreaView dragState name { x, y, width, height } target =
    let
        active =
            case dragState of
                Dragging team _ ->
                    team == name

                _ ->
                    False
    in
    Html.div
        [ classList
            [ ( "drop-area", True )
            , ( "active", active )
            ]
        , style "position" "absolute"
        , style "transform" <|
            "translate("
                ++ String.fromFloat x
                ++ "px,"
                ++ String.fromFloat y
                ++ "px)"
        , style "width" <| String.fromFloat width ++ "px"
        , style "height" <| String.fromFloat height ++ "px"
        , on "dragenter" (Json.Decode.succeed (DragOver target))

        -- preventDefault is required so that the card will not appear to
        -- "float" or "snap" back to its original position when dropped.
        , preventDefaultOn "dragover" (Json.Decode.succeed ( DragOver target, True ))
        , stopPropagationOn "drop" (Json.Decode.succeed ( DragEnd, True ))
        ]
        []


headerView : Grid.Bounds -> String -> Html Message
headerView { x, y, width, height } header =
    Html.div
        [ class "header"
        , style "position" "absolute"
        , style "transform" <|
            "translate("
                ++ String.fromFloat x
                ++ "px,"
                ++ String.fromFloat y
                ++ "px)"
        , style "width" <| String.fromFloat width ++ "px"
        , style "height" <| String.fromFloat height ++ "px"
        , style "font-size" "18px"
        , style "padding-left" "12.5px"
        , style "padding-top" "17.5px"
        , style "box-sizing" "border-box"
        , style "text-overflow" "ellipsis"
        , style "overflow" "hidden"
        , style "white-space" "nowrap"
        , style "font-weight" Views.Styles.fontWeightBold
        ]
        [ Html.text header ]
