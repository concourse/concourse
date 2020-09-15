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
import Dashboard.Group.Models exposing (Card(..), Pipeline)
import Dashboard.Group.Tag as Tag
import Dashboard.InstanceGroup as InstanceGroup
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


ordering : { a | userState : UserState } -> Ordering Concourse.TeamName
ordering session =
    Ordering.byFieldWith Tag.ordering (tag session)
        |> Ordering.breakTiesWith Ordering.natural


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
        , dropAreas : List Grid.DropArea
        , groupCardsHeight : Float
        , pipelineJobs : Dict Concourse.DatabaseID (List Concourse.JobIdentifier)
        , jobs : Dict ( Concourse.DatabaseID, String ) Concourse.Job
        }
    -> Concourse.TeamName
    -> List Grid.Card
    -> Html Message
view session params teamName cards =
    let
        cardViews =
            if List.isEmpty cards then
                [ ( "not-set", Pipeline.pipelineNotSetView ) ]

            else
                cards
                    |> List.map
                        (\{ bounds, card } ->
                            case card of
                                PipelineCard pipeline ->
                                    pipelineCardView session
                                        params
                                        AllPipelinesSection
                                        { bounds = bounds, pipeline = pipeline }
                                        teamName
                                        |> (\html -> ( String.fromInt pipeline.id, html ))

                                InstanceGroupCard p ps ->
                                    instanceGroupCardView session
                                        params
                                        AllPipelinesSection
                                        bounds
                                        p
                                        ps
                                        |> (\html -> ( p.name, html ))
                        )

        dropAreaViews =
            params.dropAreas
                |> List.map
                    (\{ bounds, target } ->
                        pipelineDropAreaView params.dragState teamName bounds target
                    )
    in
    Html.div
        [ id <| Effects.toHtmlID <| DashboardGroup teamName
        , class "dashboard-team-group"
        , attribute "data-team-name" teamName
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
                [ Html.text teamName ]
                :: (Maybe.Extra.toList <|
                        Maybe.map (Tag.view False) (tag session teamName)
                   )
                ++ (if params.dropState == DroppingWhileApiRequestInFlight teamName then
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
            (cardViews ++ [ ( "drop-areas", Html.div [ style "position" "absolute" ] dropAreaViews ) ])
        ]


viewFavoritePipelines :
    Session
    ->
        { dragState : DragState
        , dropState : DropState
        , now : Maybe Time.Posix
        , pipelinesWithResourceErrors : Set Concourse.DatabaseID
        , pipelineLayers : Dict Concourse.DatabaseID (List (List Concourse.JobIdentifier))
        , groupCardsHeight : Float
        , pipelineJobs : Dict Concourse.DatabaseID (List Concourse.JobIdentifier)
        , jobs : Dict ( Concourse.DatabaseID, String ) Concourse.Job
        }
    -> List Grid.Header
    -> List Grid.Card
    -> Html Message
viewFavoritePipelines session params headers cards =
    let
        cardViews =
            cards
                |> List.map
                    (\{ bounds, card } ->
                        case card of
                            PipelineCard pipeline ->
                                pipelineCardView session
                                    params
                                    FavoritesSection
                                    { bounds = bounds, pipeline = pipeline }
                                    pipeline.teamName
                                    |> (\html -> ( String.fromInt pipeline.id, html ))

                            InstanceGroupCard p ps ->
                                instanceGroupCardView session
                                    params
                                    FavoritesSection
                                    bounds
                                    p
                                    ps
                                    |> (\html -> ( p.name, html ))
                    )

        headerViews =
            headers
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
        (cardViews
            ++ [ ( "headers"
                 , Html.div
                    [ style "position" "absolute"
                    , class "headers"
                    ]
                    headerViews
                 )
               ]
        )


tag : { a | userState : UserState } -> Concourse.TeamName -> Maybe Tag.Tag
tag { userState } teamName =
    case userState of
        UserStateLoggedIn user ->
            Tag.tag user teamName

        _ ->
            Nothing


hdView :
    { pipelineRunningKeyframes : String
    , pipelinesWithResourceErrors : Set Concourse.DatabaseID
    , pipelineJobs : Dict Concourse.DatabaseID (List Concourse.JobIdentifier)
    , jobs : Dict ( Concourse.DatabaseID, String ) Concourse.Job
    }
    -> { a | userState : UserState }
    -> ( Concourse.TeamName, List Card )
    -> List (Html Message)
hdView { pipelineRunningKeyframes, pipelinesWithResourceErrors, pipelineJobs, jobs } session ( teamName, cards ) =
    let
        header =
            Html.div
                [ class "dashboard-team-name" ]
                [ Html.text teamName ]
                :: (Maybe.Extra.toList <| Maybe.map (Tag.view True) (tag session teamName))

        teamPipelines =
            if List.isEmpty cards then
                [ pipelineNotSetView ]

            else
                cards
                    |> List.map
                        (\card ->
                            case card of
                                PipelineCard p ->
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

                                InstanceGroupCard p ps ->
                                    InstanceGroup.hdCardView
                                        { pipeline = p
                                        , pipelines = ps
                                        , resourceError =
                                            List.any
                                                (\pipeline ->
                                                    Set.member pipeline.id pipelinesWithResourceErrors
                                                )
                                                (p :: ps)
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
        [ class "card no-pipelines-card" ]
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
        ([ class "card-wrapper"
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
            ([ class "card pipeline-card"
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


instanceGroupCardView :
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
    -> Grid.Bounds
    -> Pipeline
    -> List Pipeline
    -> Html Message
instanceGroupCardView session params section bounds p ps =
    Html.div
        ([ class "card-wrapper"
         , style "position" "absolute"
         , style "transform"
            ("translate("
                ++ String.fromFloat bounds.x
                ++ "px,"
                ++ String.fromFloat bounds.y
                ++ "px)"
            )
         , style "width" (String.fromFloat bounds.width ++ "px")
         , style "height" (String.fromFloat bounds.height ++ "px")
         , onMouseOver <|
            Hover <|
                Just <|
                    -- TODO: no
                    PipelineWrapper p.id
         , onMouseOut <| Hover Nothing
         ]
            ++ (if params.dragState /= NotDragging then
                    [ style "transition" "transform 0.2s ease-in-out" ]

                else
                    []
               )
            ++ (let
                    hoverStyle id =
                        if id == p.id then
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
            ([ class "card instance-group-card"
             , style "width" "100%"
             , style "height" "100%"

             -- TODO: yes?
             , attribute "data-pipeline-name" p.name
             ]
                -- TODO: no
                ++ (if section == AllPipelinesSection && not p.stale then
                        [ attribute
                            "ondragstart"
                            "event.dataTransfer.setData('text/plain', '');"
                        , draggable "true"
                        , on "dragstart"
                            (Json.Decode.succeed (DragStart p.teamName p.name))
                        , on "dragend" (Json.Decode.succeed DragEnd)
                        ]

                    else
                        []
                   )
                ++ (if params.dragState == Dragging p.teamName p.name then
                        [ style "width" "0"
                        , style "margin" "0 12.5px"
                        , style "overflow" "hidden"
                        ]

                    else
                        []
                   )
                ++ (if params.dropState == DroppingWhileApiRequestInFlight p.teamName then
                        [ style "opacity" "0.45", style "pointer-events" "none" ]

                    else
                        [ style "opacity" "1" ]
                   )
            )
            [ InstanceGroup.cardView session
                { pipeline = p
                , pipelines = ps
                , resourceError =
                    List.any
                        (\pipeline ->
                            Set.member pipeline.id params.pipelinesWithResourceErrors
                        )
                        (p :: ps)
                , pipelineJobs = params.pipelineJobs
                , jobs = params.jobs
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
