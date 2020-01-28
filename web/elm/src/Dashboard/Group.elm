module Dashboard.Group exposing
    ( PipelineIndex
    , hdView
    , ordering
    , pipelineDropAreaView
    , pipelineNotSetView
    , view
    )

import Concourse
import Dashboard.DashboardPreview as DashboardPreview
import Dashboard.Group.Models exposing (Group, Pipeline)
import Dashboard.Group.Tag as Tag
import Dashboard.Models exposing (DragState(..), DropState(..))
import Dashboard.Pipeline as Pipeline
import Dashboard.PipelineGridLayout as PipelineGridLayout
import Dashboard.Styles as Styles
import Dict exposing (Dict)
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, draggable, id, style)
import Html.Events exposing (on, preventDefaultOn, stopPropagationOn)
import Json.Decode
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
        }
    -> Group
    -> ( Html Message, Float )
view session params g =
    let
        pipelinesForGroup =
            g.pipelines

        padding =
            25

        headerHeight =
            60

        cardWidth =
            272

        cardHeight =
            268

        numColumns =
            max 1 (floor (params.viewportWidth / (cardWidth + padding)))

        numRowsVisible =
            max 1 (ceiling ((params.viewportHeight - headerHeight) / (cardHeight + padding)))

        numRowsOffset =
            floor ((params.scrollTop + headerHeight) / (cardHeight + padding))

        isVisible { row, height } =
            (numRowsOffset < row + height)
                && (row <= numRowsOffset + numRowsVisible)

        paddingElem =
            Html.div
                [ style "grid-row"
                    ("1 / span " ++ String.fromInt numRows)
                , style "grid-column" "1 / span 1"
                , style "width" "1px"
                ]
                []

        layersList =
            pipelinesForGroup
                |> List.map
                    (\pipeline ->
                        Dict.get ( pipeline.name, pipeline.teamName ) params.pipelineLayers
                            |> Maybe.withDefault []
                    )

        previewSizes =
            layersList
                |> List.map
                    (\layers ->
                        ( List.length layers
                        , layers
                            |> List.map List.length
                            |> List.maximum
                            |> Maybe.withDefault 0
                        )
                    )

        cards =
            previewSizes
                |> List.map PipelineGridLayout.cardSize
                |> PipelineGridLayout.layout numColumns

        numRows =
            cards
                |> List.map (\c -> c.row + c.height - 1)
                |> List.maximum
                |> Maybe.withDefault 1

        pipelineCards =
            if List.isEmpty pipelinesForGroup then
                [ Pipeline.pipelineNotSetView ]

            else
                paddingElem
                    :: List.append
                        (pipelinesForGroup
                            |> List.map3
                                (\card layers pipeline ->
                                    ( card, layers, pipeline )
                                )
                                cards
                                layersList
                            |> List.filter (\( card, _, _ ) -> isVisible card)
                            |> List.map
                                (\( card, layers, pipeline ) ->
                                    Html.div
                                        [ class "pipeline-wrapper"
                                        , style "grid-column" (String.fromInt card.column ++ " / span " ++ String.fromInt card.width)
                                        , style "grid-row" (String.fromInt card.row ++ " / span " ++ String.fromInt card.height)
                                        ]
                                        [ pipelineDropAreaView params.dragState params.dropState g.teamName pipeline.ordering
                                        , Html.div
                                            ([ class "card"
                                             , style "width" "100%"
                                             , id <| Effects.toHtmlID <| PipelineCard pipeline.id
                                             , attribute "data-pipeline-name" pipeline.name
                                             , attribute
                                                "ondragstart"
                                                "event.dataTransfer.setData('text/plain', '');"
                                             , draggable "true"
                                             , on "dragstart"
                                                (Json.Decode.succeed (DragStart pipeline.teamName pipeline.ordering))
                                             , on "dragend" (Json.Decode.succeed DragEnd)
                                             ]
                                                ++ (if params.dragState == Dragging pipeline.teamName pipeline.ordering then
                                                        [ style "width" "0"
                                                        , style "margin" "0 12.5px"
                                                        , style "overflow" "hidden"
                                                        ]

                                                    else
                                                        []
                                                   )
                                                ++ (if params.dropState == DroppingWhileApiRequestInFlight g.teamName then
                                                        [ style "opacity" "0.45", style "pointer-events" "none" ]

                                                    else
                                                        [ style "opacity" "1" ]
                                                   )
                                            )
                                            [ Pipeline.pipelineView
                                                { now = params.now
                                                , pipeline = pipeline
                                                , resourceError =
                                                    params.pipelinesWithResourceErrors
                                                        |> Dict.get ( pipeline.teamName, pipeline.name )
                                                        |> Maybe.withDefault False
                                                , existingJobs =
                                                    params.existingJobs
                                                        |> List.filter
                                                            (\j ->
                                                                j.teamName == pipeline.teamName && j.pipelineName == pipeline.name
                                                            )
                                                , layers = layers
                                                , hovered = params.hovered
                                                , pipelineRunningKeyframes = params.pipelineRunningKeyframes
                                                , userState = session.userState
                                                }
                                            ]
                                        ]
                                )
                        )
                        [ pipelineDropAreaView params.dragState
                            params.dropState
                            g.teamName
                            (pipelinesForGroup
                                |> List.map (.ordering >> (+) 1)
                                |> List.maximum
                                |> Maybe.withDefault 0
                            )
                        ]
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
        , Html.div
            [ class <| .sectionBodyClass Effects.stickyHeaderConfig
            , style "display" "grid"
            , style "grid-template-columns" <|
                "repeat("
                    ++ String.fromInt numColumns
                    ++ ","
                    ++ String.fromInt cardWidth
                    ++ "px)"
            , style "grid-template-rows" <|
                "repeat("
                    ++ String.fromInt numRows
                    ++ ","
                    ++ String.fromInt cardHeight
                    ++ "px)"
            , style "grid-gap" <| String.fromInt padding ++ "px"
            ]
            pipelineCards
        ]
        |> (\html ->
                ( html
                , toFloat <|
                    numRows
                        * cardHeight
                        + padding
                        * (numRows - 1)
                        + headerHeight
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
        pipelinesForGroup =
            g.pipelines

        header =
            Html.div
                [ class "dashboard-team-name" ]
                [ Html.text g.teamName ]
                :: (Maybe.Extra.toList <| Maybe.map (Tag.view True) (tag session g))

        teamPipelines =
            if List.isEmpty pipelinesForGroup then
                [ pipelineNotSetView ]

            else
                pipelinesForGroup
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


pipelineDropAreaView : DragState -> DropState -> String -> Int -> Html Message
pipelineDropAreaView dragState dropState name index =
    let
        ( active, over ) =
            case ( dragState, dropState ) of
                ( Dragging team dragIdx, NotDropping ) ->
                    ( team == name, index == dragIdx )

                ( Dragging team _, Dropping dropIdx ) ->
                    ( team == name, index == dropIdx )

                _ ->
                    ( False, False )
    in
    Html.div
        [ classList
            [ ( "drop-area", True )
            , ( "active", active )
            , ( "animation", dropState /= NotDropping )
            ]
        , on "dragenter" (Json.Decode.succeed (DragOver name index))

        -- preventDefault is required so that the card will not appear to
        -- "float" or "snap" back to its original position when dropped.
        , preventDefaultOn "dragover" (Json.Decode.succeed ( DragOver name index, True ))
        , stopPropagationOn "drop" (Json.Decode.succeed ( DragEnd, True ))
        , style "padding" <|
            "0 "
                ++ (if active && over then
                        "198.5px"

                    else
                        "50px"
                   )
        ]
        []
