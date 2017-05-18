module SideBar exposing (Model, Msg(..), init, update, view, subscriptions, fetchPipelines)

import Html exposing (Html)
import Html.Attributes exposing (class, href, id, disabled, attribute, style)
import Html.Events as Events
import Http
import List
import Mouse exposing (Position)
import Navigation
import Task
import Concourse
import LoginRedirect
import Concourse.Pipeline
import StrictEvents exposing (onLeftClick, onLeftMouseDownCapturing)


type alias Model =
    { teams : Maybe (List ( String, List UIPipeline ))
    , dragInfo : Maybe DragInfo
    , csrfToken : String
    }


purposefulThreshold : Int
purposefulThreshold =
    10



-- in pixels


type alias DragInfo =
    { startPos : Position
    , pos : Position
    , teamName : String
    , pipelineName : String
    , purposeful : Bool
    , hover : Maybe (ListHover String)
    }


type alias UIPipeline =
    { pipeline : Concourse.Pipeline
    , pausedChanging : Bool
    , pauseErrored : Bool
    }


type ListHover a
    = BeforeAll
    | AfterElement a


type Msg
    = Noop
    | PausePipeline String String
    | UnpausePipeline String String
    | PipelinesFetched (Result Http.Error (List Concourse.Pipeline))
    | PipelinePaused String String (Result Http.Error ())
    | PipelineUnpaused String String (Result Http.Error ())
    | StartDragging String String Position
    | StopDragging Position
    | Drag Position
    | Hover String (ListHover String)
    | Unhover String (ListHover String)
    | PipelinesReordered String (Result Http.Error ())
    | NavToPipeline String
    | NewCSRFToken String


type alias Flags =
    { csrfToken : String }


init : Flags -> ( Model, Cmd Msg )
init flags =
    ( { teams = Nothing
      , dragInfo = Nothing
      , csrfToken = flags.csrfToken
      }
    , fetchPipelines
    )


subscriptions : Model -> Sub Msg
subscriptions model =
    if isDragging model then
        Sub.batch [ Mouse.moves Drag, Mouse.ups StopDragging ]
    else
        Sub.none


update : Msg -> Model -> ( Model, Cmd Msg )
update action model =
    case action of
        Noop ->
            ( model, Cmd.none )

        NewCSRFToken csrfToken ->
            ( { model
                | csrfToken = csrfToken
              }
            , Cmd.none
            )

        PausePipeline teamName pipelineName ->
            ( mapModelPipelines updatePausedChanging teamName pipelineName model
            , pausePipeline teamName pipelineName model.csrfToken
            )

        UnpausePipeline teamName pipelineName ->
            ( mapModelPipelines updatePausedChanging teamName pipelineName model
            , unpausePipeline teamName pipelineName model.csrfToken
            )

        PipelinesFetched (Ok pipelines) ->
            ( { model | teams = Just <| groupPipelinesByTeam pipelines }, Cmd.none )

        PipelinesFetched (Err err) ->
            flip always (Debug.log ("failed to fetch pipelines") (err)) <|
                ( model, Cmd.none )

        PipelinePaused teamName pipelineName (Ok ()) ->
            ( mapModelPipelines (setPaused True) teamName pipelineName model
            , Cmd.none
            )

        PipelinePaused teamName pipelineName (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 || status.code == 403 then
                        ( model, LoginRedirect.requestLoginRedirect "" )
                    else
                        ( model, Cmd.none )

                _ ->
                    ( mapModelPipelines updatePauseErrored teamName pipelineName model
                    , Cmd.none
                    )

        PipelineUnpaused teamName pipelineName (Ok ()) ->
            ( mapModelPipelines (setPaused False) teamName pipelineName model
            , Cmd.none
            )

        PipelineUnpaused teamName pipelineName (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 || status.code == 403 then
                        ( model, LoginRedirect.requestLoginRedirect "" )
                    else
                        ( model, Cmd.none )

                _ ->
                    ( mapModelPipelines updatePauseErrored teamName pipelineName model
                    , Cmd.none
                    )

        Drag pos ->
            case model.dragInfo of
                Just dragInfo ->
                    ( { model
                        | dragInfo =
                            Just <|
                                let
                                    newDragInfo =
                                        { dragInfo | pos = pos }
                                in
                                    { newDragInfo
                                        | purposeful =
                                            dragInfo.purposeful
                                                || abs (dragX newDragInfo)
                                                >= purposefulThreshold
                                                || abs (dragY newDragInfo)
                                                >= purposefulThreshold
                                    }
                      }
                    , Cmd.none
                    )

                Nothing ->
                    ( model, Cmd.none )

        StartDragging teamName pipelineName pos ->
            ( { model
                | dragInfo =
                    Just
                        { startPos = pos
                        , pos = pos
                        , teamName = teamName
                        , pipelineName = pipelineName
                        , purposeful = False
                        , hover = Nothing
                        }
              }
            , Cmd.none
            )

        StopDragging pos ->
            case model.dragInfo of
                Just dragInfo ->
                    case dragInfo.hover of
                        Just hover ->
                            case getPipelinesByTeamName dragInfo.teamName model.teams of
                                Just pipelines ->
                                    case popPipelineByName dragInfo.pipelineName pipelines of
                                        ( tmpPipelines, Just draggedPipeline ) ->
                                            let
                                                updatedPipelines =
                                                    insertPipelineAt hover draggedPipeline tmpPipelines
                                            in
                                                ( { model
                                                    | dragInfo = Nothing
                                                    , teams =
                                                        setPipelinesByTeamName
                                                            dragInfo.teamName
                                                            updatedPipelines
                                                            model.teams
                                                  }
                                                , orderPipelines
                                                    dragInfo.teamName
                                                    (List.map (.pipeline >> .name) updatedPipelines)
                                                    model.csrfToken
                                                )

                                        ( _, Nothing ) ->
                                            ( { model | dragInfo = Nothing }, Cmd.none )

                                Nothing ->
                                    ( { model | dragInfo = Nothing }, Cmd.none )

                        Nothing ->
                            ( { model | dragInfo = Nothing }, Cmd.none )

                Nothing ->
                    ( model, Cmd.none )

        Hover teamName listHover ->
            case model.dragInfo of
                Just dragInfo ->
                    if
                        dragInfo.teamName
                            == teamName
                            && listHover
                            /= AfterElement dragInfo.pipelineName
                            && (getPrevHover model)
                            /= Just listHover
                    then
                        ( { model
                            | dragInfo = Just { dragInfo | hover = Just listHover }
                          }
                        , Cmd.none
                        )
                    else
                        ( model, Cmd.none )

                Nothing ->
                    ( model, Cmd.none )

        Unhover teamName listHover ->
            case model.dragInfo of
                Just dragInfo ->
                    if dragInfo.hover == Just listHover then
                        ( { model
                            | dragInfo = Just { dragInfo | hover = Nothing }
                          }
                        , Cmd.none
                        )
                    else
                        ( model, Cmd.none )

                Nothing ->
                    ( model, Cmd.none )

        PipelinesReordered teamName (Ok ()) ->
            ( model, Cmd.none )

        PipelinesReordered teamName (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, LoginRedirect.requestLoginRedirect "" )
                    else
                        ( model, Cmd.none )

                _ ->
                    ( model, Cmd.none )

        NavToPipeline url ->
            ( model, Navigation.newUrl url )


getPrevHover : Model -> Maybe (ListHover String)
getPrevHover model =
    case model.dragInfo of
        Just dragInfo ->
            Maybe.andThen
                (getPrevHoverForPipelines dragInfo.pipelineName)
                (getPipelinesByTeamName dragInfo.teamName model.teams)

        Nothing ->
            Nothing


getPrevHoverForPipelines : String -> List UIPipeline -> Maybe (ListHover String)
getPrevHoverForPipelines pipelineName pipelines =
    case pipelines of
        [] ->
            Nothing

        [ first ] ->
            Just BeforeAll

        first :: second :: rest ->
            if second.pipeline.name == pipelineName then
                Just <| AfterElement first.pipeline.name
            else
                getPrevHoverForPipelines pipelineName <| second :: rest


getPipelinesByTeamName : String -> Maybe (List ( String, List UIPipeline )) -> Maybe (List UIPipeline)
getPipelinesByTeamName teamName teams =
    Maybe.map
        Tuple.second
    <|
        List.head <|
            List.filter
                (\team -> Tuple.first team == teamName)
            <|
                Maybe.withDefault [] teams


setPipelinesByTeamName :
    String
    -> List UIPipeline
    -> Maybe (List ( String, List UIPipeline ))
    -> Maybe (List ( String, List UIPipeline ))
setPipelinesByTeamName teamName newPipelines teams =
    Maybe.map (setPipelinesByTeamNameHelper teamName newPipelines) teams


setPipelinesByTeamNameHelper :
    String
    -> List UIPipeline
    -> List ( String, List UIPipeline )
    -> List ( String, List UIPipeline )
setPipelinesByTeamNameHelper teamName newPipelines teams =
    case teams of
        [] ->
            []

        team :: rest ->
            if Tuple.first team == teamName then
                ( Tuple.first team, newPipelines ) :: rest
            else
                team :: setPipelinesByTeamNameHelper teamName newPipelines rest


popPipelineByName : String -> List UIPipeline -> ( List UIPipeline, Maybe UIPipeline )
popPipelineByName name pipelines =
    case pipelines of
        [] ->
            ( [], Nothing )

        first :: rest ->
            if first.pipeline.name == name then
                ( rest, Just first )
            else
                let
                    ( updatedRest, poppedPipeline ) =
                        popPipelineByName name rest
                in
                    ( first :: updatedRest, poppedPipeline )


insertPipelineAt : ListHover String -> UIPipeline -> List UIPipeline -> List UIPipeline
insertPipelineAt location pipeline pipelines =
    case location of
        BeforeAll ->
            pipeline :: pipelines

        AfterElement name ->
            case pipelines of
                [] ->
                    []

                first :: rest ->
                    if first.pipeline.name == name then
                        first :: pipeline :: rest
                    else
                        first :: (insertPipelineAt location pipeline rest)


dragX : DragInfo -> Int
dragX dragInfo =
    dragInfo.pos.x - dragInfo.startPos.x


dragY : DragInfo -> Int
dragY dragInfo =
    dragInfo.pos.y - dragInfo.startPos.y


isDragging : Model -> Bool
isDragging model =
    model.dragInfo /= Nothing


isPurposeful : Maybe DragInfo -> Bool
isPurposeful =
    Maybe.withDefault False << Maybe.map .purposeful


setPaused : Bool -> UIPipeline -> UIPipeline
setPaused paused uip =
    -- arbitrary elm restriction: record update syntax only works on local variables
    let
        pipeline =
            uip.pipeline
    in
        { uip
            | pipeline = { pipeline | paused = paused }
            , pausedChanging = False
            , pauseErrored = False
        }


updatePausedChanging : UIPipeline -> UIPipeline
updatePausedChanging uip =
    { uip | pausedChanging = True }


updatePauseErrored : UIPipeline -> UIPipeline
updatePauseErrored uip =
    { uip | pauseErrored = True, pausedChanging = False }


mapModelPipelines : (UIPipeline -> UIPipeline) -> String -> String -> Model -> Model
mapModelPipelines f teamName pipelineName model =
    let
        maybePipelines =
            getPipelinesByTeamName teamName model.teams
    in
        case maybePipelines of
            Nothing ->
                model

            Just pipelines ->
                let
                    newPipelines =
                        List.map (mapPipeline f pipelineName) pipelines
                in
                    { model
                        | teams = setPipelinesByTeamName teamName newPipelines model.teams
                    }


mapPipeline : (UIPipeline -> UIPipeline) -> String -> UIPipeline -> UIPipeline
mapPipeline f pipelineName uip =
    if uip.pipeline.name == pipelineName then
        f uip
    else
        uip


view : Model -> Html Msg
view model =
    case model.teams of
        Nothing ->
            Html.text "loading"

        Just teams ->
            Html.ul [] <| List.map (viewTeam model.dragInfo) teams


viewTeam : Maybe DragInfo -> ( String, List UIPipeline ) -> Html Msg
viewTeam maybeDragInfo ( teamName, pipelines ) =
    Html.li [ class "team" ]
        [ Html.div [ class "team-header" ]
            [ Html.text <| "team "
            , Html.span [ class "bright-text" ]
                [ Html.text teamName ]
            ]
        , Html.ul [] <|
            let
                firstElem =
                    List.head pipelines
            in
                case firstElem of
                    Nothing ->
                        []

                    Just firstElem ->
                        let
                            firstElemView =
                                viewFirstPipeline maybeDragInfo firstElem
                        in
                            let
                                restView =
                                    List.map
                                        (viewPipeline maybeDragInfo)
                                    <|
                                        Maybe.withDefault [] <|
                                            List.tail pipelines
                            in
                                firstElemView :: restView
        ]


viewFirstPipeline : Maybe DragInfo -> UIPipeline -> Html Msg
viewFirstPipeline maybeDragInfo uip =
    Html.li
        (case maybeDragInfo of
            Just dragInfo ->
                if dragInfo.teamName == uip.pipeline.teamName then
                    case dragInfo.hover of
                        Just hover ->
                            if hover == BeforeAll then
                                [ class "space-before" ]
                            else if hover == AfterElement uip.pipeline.name then
                                [ class "space-after" ]
                            else
                                []

                        Nothing ->
                            []
                else
                    []

            Nothing ->
                []
        )
    <|
        (if isPurposeful maybeDragInfo then
            [ viewFirstDropArea uip.pipeline.teamName
            , viewDropArea uip.pipeline.teamName uip.pipeline.name
            ]
         else
            []
        )
            ++ [ viewDraggable maybeDragInfo uip ]


viewPipeline : Maybe DragInfo -> UIPipeline -> Html Msg
viewPipeline maybeDragInfo uip =
    Html.li
        (case maybeDragInfo of
            Just dragInfo ->
                if dragInfo.teamName == uip.pipeline.teamName then
                    case dragInfo.hover of
                        Just hover ->
                            if hover == AfterElement uip.pipeline.name then
                                [ class "space-after" ]
                            else
                                []

                        Nothing ->
                            []
                else
                    []

            Nothing ->
                []
        )
    <|
        (if isPurposeful maybeDragInfo then
            [ viewDropArea uip.pipeline.teamName uip.pipeline.name ]
         else
            []
        )
            ++ [ viewDraggable maybeDragInfo uip ]


viewDraggable : Maybe DragInfo -> UIPipeline -> Html Msg
viewDraggable maybeDragInfo uip =
    Html.div
        (let
            dragging =
                case maybeDragInfo of
                    Just dragInfo ->
                        dragInfo.teamName
                            == uip.pipeline.teamName
                            && dragInfo.pipelineName
                            == uip.pipeline.name

                    Nothing ->
                        False
         in
            [ class <|
                if dragging then
                    if isPurposeful maybeDragInfo then
                        "draggable dragging purposeful"
                    else
                        "draggable dragging"
                else
                    "draggable"
            , onLeftMouseDownCapturing Mouse.position <|
                StartDragging uip.pipeline.teamName uip.pipeline.name
            ]
                ++ case ( maybeDragInfo, dragging ) of
                    ( Just dragInfo, True ) ->
                        [ dragStyle dragInfo ]

                    _ ->
                        []
        )
        [ Html.div []
            [ viewPauseButton uip
            , Html.a
                [ class "pipeline"
                , onLeftClick <| NavToPipeline uip.pipeline.url
                , href uip.pipeline.url
                ]
                [ Html.text uip.pipeline.name ]
            ]
        ]


dragStyle : DragInfo -> Html.Attribute action
dragStyle dragInfo =
    style
        [ ( "top", toString (dragY dragInfo) ++ "px" )
        ]


viewFirstDropArea : String -> Html Msg
viewFirstDropArea teamName =
    Html.div
        [ class "drop-area first"
        , Events.onMouseEnter <| Hover teamName BeforeAll
        , Events.onMouseLeave <| Unhover teamName BeforeAll
        ]
        []


viewDropArea : String -> String -> Html Msg
viewDropArea teamName pipelineName =
    Html.div
        [ class "drop-area"
        , Events.onMouseEnter <| Hover teamName <| AfterElement pipelineName
        , Events.onMouseLeave <| Unhover teamName <| AfterElement pipelineName
        ]
        []


viewPauseButton : UIPipeline -> Html Msg
viewPauseButton uip =
    if uip.pipeline.paused then
        Html.a
            [ Events.onClick <| UnpausePipeline uip.pipeline.teamName uip.pipeline.name
            , class <|
                if uip.pauseErrored then
                    "btn-pause errored"
                else
                    "btn-pause enabled"
            ]
        <|
            if uip.pausedChanging then
                [ Html.i [ class "fa fa-fw fa-circle-o-notch fa-spin" ] [] ]
            else
                [ Html.i [ class "fa fa-fw fa-play" ] [] ]
    else
        Html.a
            [ Events.onClick <| PausePipeline uip.pipeline.teamName uip.pipeline.name
            , class <|
                if uip.pauseErrored then
                    "btn-pause errored"
                else
                    "btn-pause disabled"
            ]
        <|
            if uip.pausedChanging then
                [ Html.i [ class "fa fa-fw fa-circle-o-notch fa-spin" ] [] ]
            else
                [ Html.i [ class "fa fa-fw fa-pause" ] [] ]


fetchPipelines : Cmd Msg
fetchPipelines =
    Task.attempt PipelinesFetched Concourse.Pipeline.fetchPipelines


unpausePipeline : String -> String -> Concourse.CSRFToken -> Cmd Msg
unpausePipeline teamName pipelineName csrfToken =
    Task.attempt (PipelineUnpaused teamName pipelineName) <|
        Concourse.Pipeline.unpause teamName pipelineName csrfToken


pausePipeline : String -> String -> Concourse.CSRFToken -> Cmd Msg
pausePipeline teamName pipelineName csrfToken =
    Task.attempt (PipelinePaused teamName pipelineName) <|
        Concourse.Pipeline.pause teamName pipelineName csrfToken


orderPipelines : String -> List String -> Concourse.CSRFToken -> Cmd Msg
orderPipelines teamName pipelineNames csrfToken =
    Task.attempt (PipelinesReordered teamName) <|
        Concourse.Pipeline.order teamName pipelineNames csrfToken


groupPipelinesByTeam : List Concourse.Pipeline -> List ( String, List UIPipeline )
groupPipelinesByTeam pipelines =
    let
        firstPipeline =
            List.head pipelines
    in
        case firstPipeline of
            Nothing ->
                []

            Just firstPipeline ->
                let
                    ( teamGroup, rest ) =
                        List.partition (\p -> p.teamName == firstPipeline.teamName) pipelines
                in
                    let
                        team =
                            List.map toUIPipeline teamGroup
                    in
                        ( firstPipeline.teamName, team ) :: (groupPipelinesByTeam rest)


toUIPipeline : Concourse.Pipeline -> UIPipeline
toUIPipeline pipeline =
    { pipeline = pipeline
    , pausedChanging = False
    , pauseErrored = False
    }
