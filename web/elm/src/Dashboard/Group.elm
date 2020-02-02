module Dashboard.Group exposing
    ( PipelineIndex
    , hdView
    , ordering
    , pipelineNotSetView
    , view
    )

import Concourse
import Dashboard.Group.Models exposing (Group, Pipeline)
import Dashboard.Group.Tag as Tag
import Dashboard.Models exposing (DragState(..), DropState(..))
import Dashboard.Pipeline as Pipeline
import Dashboard.PipelineGrid as PipelineGrid
import Dashboard.Styles as Styles
import Dict exposing (Dict)
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, id, style)
import Maybe.Extra
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import Ordering exposing (Ordering)
import Time
import UserState exposing (UserState(..))
import Views.Spinner as Spinner


ordering : { a | userState : UserState } -> Ordering Group
ordering session =
    Ordering.byFieldWith Tag.ordering (tag session)
        |> Ordering.breakTiesWith (Ordering.byField .teamName)


type alias PipelineIndex =
    Int


view :
    { a | userState : UserState }
    ->
        { dragState : DragState
        , dropState : DropState
        , now : Maybe Time.Posix
        , hovered : HoverState.HoverState
        , pipelineRunningKeyframes : String
        , pipelinesWithResourceErrors : Dict ( String, String ) Bool
        , existingJobs : List Concourse.Job
        , pipelineLayers : Dict ( String, String ) (List (List Concourse.Job))
        , viewportWidth : Float
        , viewportHeight : Float
        , scrollTop : Float
        , query : String
        }
    -> Group
    -> ( Html Message, Float )
view session params g =
    let
        ( grid, gridHeight ) =
            PipelineGrid.view session params g

        headerHeight =
            60
    in
    Html.div
        [ id <| Effects.toHtmlID <| DashboardGroup g.teamName
        , class "dashboard-team-group"
        , attribute "data-team-name" g.teamName
        ]
        [ Html.div
            [ style "display" "flex"
            , style "align-items" "center"
            , class <| .sectionHeaderClass Effects.stickyHeaderConfig
            ]
            (Html.div
                [ class "dashboard-team-name" ]
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
        , grid
        ]
        |> (\html ->
                ( html
                , gridHeight + headerHeight
                )
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
    , pipelinesWithResourceErrors : Dict ( String, String ) Bool
    , existingJobs : List Concourse.Job
    }
    -> { a | userState : UserState }
    -> Group
    -> List (Html Message)
hdView { pipelineRunningKeyframes, pipelinesWithResourceErrors, existingJobs } session g =
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
                                        |> Dict.get ( p.teamName, p.name )
                                        |> Maybe.withDefault False
                                , existingJobs = existingJobs
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
