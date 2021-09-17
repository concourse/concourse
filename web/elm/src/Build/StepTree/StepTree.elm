module Build.StepTree.StepTree exposing
    ( extendHighlight
    , finished
    , init
    , setAcrossSubsteps
    , setHighlight
    , setImageCheck
    , setImageGet
    , switchTab
    , toggleStep
    , toggleStepInitialization
    , toggleStepSubHeader
    , tooltip
    , view
    )

import Ansi.Log
import Array exposing (Array)
import Assets
import Build.StepTree.Models
    exposing
        ( HookedStep
        , MetadataField
        , Step
        , StepName
        , StepState(..)
        , StepTree(..)
        , StepTreeModel
        , TabFocus(..)
        , Version
        , focusTabbed
        , isActive
        , lastActive
        , mostSevereStepState
        , showStepState
        , toggleSubHeaderExpanded
        , treeIsActive
        , updateAt
        , updateTreeNodeAt
        )
import Build.Styles as Styles
import Colors
import Concourse exposing (JsonValue, flattenJson)
import DateFormat
import Dict exposing (Dict)
import Duration
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, href, id, style, target)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import List.Extra
import Maybe.Extra
import Message.Effects exposing (Effect(..), toHtmlID)
import Message.Message exposing (DomID(..), Message(..))
import Routes exposing (Highlight(..), StepID, showHighlight)
import StrictEvents
import Time
import Tooltip
import Url
import Views.DictView as DictView
import Views.Icon as Icon
import Views.Spinner as Spinner


init :
    Maybe Concourse.JobBuildIdentifier
    -> Highlight
    -> Concourse.BuildResources
    -> Concourse.BuildPlan
    -> StepTreeModel
init buildId hl resources plan =
    let
        step =
            constructStep plan
    in
    case plan.step of
        Concourse.BuildStepTask _ ->
            step |> initBottom buildId hl resources plan Task

        Concourse.BuildStepCheck _ imagePlans ->
            step
                |> initBottom buildId hl resources plan Check
                |> setImagePlans buildId step.id imagePlans

        Concourse.BuildStepGet name _ version imagePlans ->
            step
                |> setupGetStep resources name version
                |> initBottom buildId hl resources plan Get
                |> setImagePlans buildId step.id imagePlans

        Concourse.BuildStepRun _ ->
            step |> initBottom buildId hl resources plan Run

        Concourse.BuildStepPut _ _ imagePlans ->
            step
                |> initBottom buildId hl resources plan Put
                |> setImagePlans buildId step.id imagePlans

        Concourse.BuildStepArtifactInput _ ->
            step |> initBottom buildId hl resources plan ArtifactInput

        Concourse.BuildStepArtifactOutput _ ->
            step |> initBottom buildId hl resources plan ArtifactOutput

        Concourse.BuildStepSetPipeline _ _ ->
            step |> initBottom buildId hl resources plan SetPipeline

        Concourse.BuildStepLoadVar _ ->
            step |> initBottom buildId hl resources plan LoadVar

        Concourse.BuildStepInParallel plans ->
            initMultiStep buildId hl resources plan.id InParallel plans Nothing

        Concourse.BuildStepDo plans ->
            initMultiStep buildId hl resources plan.id Do plans Nothing

        Concourse.BuildStepAcross { vars, steps } ->
            let
                values =
                    List.map .values steps

                plans =
                    List.map .step steps

                expandedHeaders =
                    plans
                        |> List.indexedMap (\i p -> ( i, planIsHighlighted hl p ))
                        |> List.filter Tuple.second
                        |> Dict.fromList
            in
            step
                |> (\s ->
                        { s
                            | expandedHeaders = expandedHeaders
                            , expanded = not <| Dict.isEmpty expandedHeaders
                        }
                   )
                |> Just
                |> initMultiStep buildId hl resources plan.id (Across plan.id vars values) (Array.fromList plans)
                |> (\model ->
                        List.foldl
                            (\plan_ ->
                                updateAt plan_.id (\s -> { s | expanded = True })
                            )
                            model
                            plans
                   )

        Concourse.BuildStepRetry plans ->
            step
                |> (\s -> { s | tabFocus = startingTab hl (Array.toList plans) })
                |> Just
                |> initMultiStep buildId hl resources plan.id (Retry plan.id) plans

        Concourse.BuildStepOnSuccess hookedPlan ->
            initHookedStep buildId hl resources OnSuccess hookedPlan

        Concourse.BuildStepOnFailure hookedPlan ->
            initHookedStep buildId hl resources OnFailure hookedPlan

        Concourse.BuildStepOnAbort hookedPlan ->
            initHookedStep buildId hl resources OnAbort hookedPlan

        Concourse.BuildStepOnError hookedPlan ->
            initHookedStep buildId hl resources OnError hookedPlan

        Concourse.BuildStepEnsure hookedPlan ->
            initHookedStep buildId hl resources Ensure hookedPlan

        Concourse.BuildStepTry subPlan ->
            initWrappedStep buildId hl resources Try subPlan

        Concourse.BuildStepTimeout subPlan ->
            initWrappedStep buildId hl resources Timeout subPlan


setImagePlans : Maybe Concourse.JobBuildIdentifier -> StepID -> Maybe Concourse.ImageBuildPlans -> StepTreeModel -> StepTreeModel
setImagePlans buildId stepId imagePlans model =
    case imagePlans of
        Nothing ->
            model

        Just { check, get } ->
            model
                |> setImageCheck buildId stepId check
                |> setImageGet buildId stepId get


setImageCheck : Maybe Concourse.JobBuildIdentifier -> StepID -> Concourse.BuildPlan -> StepTreeModel -> StepTreeModel
setImageCheck buildId stepId subPlan model =
    let
        sub =
            init buildId model.highlight model.resources subPlan
    in
    { model
        | steps =
            Dict.union sub.steps model.steps
                |> Dict.update stepId (Maybe.map (\step -> { step | imageCheck = Just sub.tree }))
    }


setImageGet : Maybe Concourse.JobBuildIdentifier -> StepID -> Concourse.BuildPlan -> StepTreeModel -> StepTreeModel
setImageGet buildId stepId subPlan model =
    let
        sub =
            init buildId model.highlight model.resources subPlan
    in
    { model
        | steps =
            Dict.union sub.steps model.steps
                |> Dict.update stepId (Maybe.map (\step -> { step | imageGet = Just sub.tree }))
    }


setAcrossSubsteps : Maybe Concourse.JobBuildIdentifier -> StepID -> List Concourse.AcrossSubstep -> StepTreeModel -> StepTreeModel
setAcrossSubsteps buildId stepId substeps model =
    case Dict.get stepId model.steps of
        Just oldStep ->
            case oldStep.buildStep of
                Concourse.BuildStepAcross { vars } ->
                    let
                        newAcrossStep =
                            Concourse.BuildStepAcross
                                { vars = vars
                                , steps = substeps
                                }

                        newAcrossModel =
                            init buildId
                                model.highlight
                                model.resources
                                { id = stepId
                                , step = newAcrossStep
                                }
                    in
                    { model
                        | steps =
                            Dict.union newAcrossModel.steps model.steps
                                -- We need to keep the old step since otherwise
                                -- we'll lose any existing state for this step
                                -- (e.g. build logs)
                                |> Dict.update stepId
                                    (Maybe.map
                                        (\newStep ->
                                            { oldStep
                                                | buildStep = newStep.buildStep
                                                , expandedHeaders = newStep.expandedHeaders
                                                , expanded = newStep.expanded || oldStep.expanded
                                            }
                                        )
                                    )
                        , tree = updateTreeNodeAt stepId (always newAcrossModel.tree) model.tree
                    }

                _ ->
                    -- Should never happen
                    model

        Nothing ->
            model


planIsHighlighted : Highlight -> Concourse.BuildPlan -> Bool
planIsHighlighted hl plan =
    case hl of
        HighlightNothing ->
            False

        HighlightLine stepID _ ->
            planContainsID stepID plan

        HighlightRange stepID _ _ ->
            planContainsID stepID plan


planContainsID : StepID -> Concourse.BuildPlan -> Bool
planContainsID stepID plan =
    plan |> Concourse.mapBuildPlan .id |> List.member stepID


startingTab : Highlight -> List Concourse.BuildPlan -> TabFocus
startingTab hl plans =
    let
        idx =
            case hl of
                HighlightNothing ->
                    Nothing

                HighlightLine stepID _ ->
                    plans |> List.Extra.findIndex (planContainsID stepID)

                HighlightRange stepID _ _ ->
                    plans |> List.Extra.findIndex (planContainsID stepID)
    in
    case idx of
        Nothing ->
            Auto

        Just tab ->
            Manual tab


initBottom : Maybe Concourse.JobBuildIdentifier -> Highlight -> Concourse.BuildResources -> Concourse.BuildPlan -> (StepID -> StepTree) -> Step -> StepTreeModel
initBottom buildId hl resources plan construct step =
    { tree = construct plan.id
    , steps = Dict.singleton plan.id (expand plan hl step)
    , highlight = hl
    , resources = resources
    , buildId = buildId
    }


initMultiStep :
    Maybe Concourse.JobBuildIdentifier
    -> Highlight
    -> Concourse.BuildResources
    -> StepID
    -> (Array StepTree -> StepTree)
    -> Array Concourse.BuildPlan
    -> Maybe Step
    -> StepTreeModel
initMultiStep buildId hl resources stepId constructor plans rootStep =
    let
        inited =
            Array.map (init buildId hl resources) plans

        trees =
            Array.map .tree inited

        selfFoci =
            case rootStep of
                Nothing ->
                    Dict.empty

                Just step ->
                    Dict.singleton stepId step
    in
    { tree = constructor trees
    , steps =
        inited
            |> Array.map .steps
            |> Array.foldr Dict.union selfFoci
    , highlight = hl
    , resources = resources
    , buildId = buildId
    }


constructStep : Concourse.BuildPlan -> Step
constructStep { id, step } =
    { id = id
    , buildStep = step
    , state = StepStatePending
    , log = Ansi.Log.init Ansi.Log.Cooked
    , error = Nothing
    , expanded = False
    , version = Nothing
    , metadata = []
    , changed = False
    , timestamps = Dict.empty
    , initialize = Nothing
    , start = Nothing
    , finish = Nothing
    , tabFocus = Auto
    , expandedHeaders = Dict.empty
    , initializationExpanded = False
    , imageCheck = Nothing
    , imageGet = Nothing
    }


expand : Concourse.BuildPlan -> Highlight -> Step -> Step
expand plan hl step =
    { step
        | expanded =
            case hl of
                HighlightNothing ->
                    False

                HighlightLine stepID _ ->
                    List.member stepID (Concourse.mapBuildPlan .id plan)

                HighlightRange stepID _ _ ->
                    List.member stepID (Concourse.mapBuildPlan .id plan)
    }


initWrappedStep :
    Maybe Concourse.JobBuildIdentifier
    -> Highlight
    -> Concourse.BuildResources
    -> (StepTree -> StepTree)
    -> Concourse.BuildPlan
    -> StepTreeModel
initWrappedStep buildId hl resources create plan =
    let
        { tree, steps } =
            init buildId hl resources plan
    in
    { tree = create tree
    , steps = steps
    , highlight = hl
    , resources = resources
    , buildId = buildId
    }


initHookedStep :
    Maybe Concourse.JobBuildIdentifier
    -> Highlight
    -> Concourse.BuildResources
    -> (HookedStep -> StepTree)
    -> Concourse.HookedPlan
    -> StepTreeModel
initHookedStep buildId hl resources create hookedPlan =
    let
        stepModel =
            init buildId hl resources hookedPlan.step

        hookModel =
            init buildId hl resources hookedPlan.hook
    in
    { tree = create { step = stepModel.tree, hook = hookModel.tree }
    , steps = Dict.union stepModel.steps hookModel.steps
    , highlight = hl
    , resources = resources
    , buildId = buildId
    }


setupGetStep : Concourse.BuildResources -> StepName -> Maybe Version -> Step -> Step
setupGetStep resources name version step =
    { step
        | version = version
        , changed = isFirstOccurrence resources.inputs name
    }


isFirstOccurrence : List Concourse.BuildResourcesInput -> StepName -> Bool
isFirstOccurrence resources step =
    case resources of
        [] ->
            False

        { name, firstOccurrence } :: rest ->
            if name == step then
                firstOccurrence

            else
                isFirstOccurrence rest step


finished : StepTreeModel -> StepTreeModel
finished model =
    { model | steps = Dict.map (always finishStep) model.steps }


finishStep : Step -> Step
finishStep step =
    let
        newState =
            case step.state of
                StepStateRunning ->
                    StepStateInterrupted

                StepStatePending ->
                    StepStateCancelled

                otherwise ->
                    otherwise
    in
    { step | state = newState }


toggleStep : StepID -> StepTreeModel -> ( StepTreeModel, List Effect )
toggleStep id root =
    ( updateAt id (\step -> { step | expanded = not step.expanded }) root
    , []
    )


toggleStepInitialization : StepID -> StepTreeModel -> ( StepTreeModel, List Effect )
toggleStepInitialization id root =
    ( updateAt id (\step -> { step | initializationExpanded = not step.initializationExpanded }) root
    , []
    )


toggleStepSubHeader : StepID -> Int -> StepTreeModel -> ( StepTreeModel, List Effect )
toggleStepSubHeader id i root =
    ( updateAt id (toggleSubHeaderExpanded i) root, [] )


switchTab : StepID -> Int -> StepTreeModel -> ( StepTreeModel, List Effect )
switchTab id tab root =
    ( updateAt id (focusTabbed tab) root, [] )


setHighlight : StepID -> Int -> StepTreeModel -> ( StepTreeModel, List Effect )
setHighlight id line root =
    let
        hl =
            HighlightLine id line
    in
    ( { root | highlight = hl }, [ ModifyUrl (showHighlight hl) ] )


extendHighlight : StepID -> Int -> StepTreeModel -> ( StepTreeModel, List Effect )
extendHighlight id line root =
    let
        hl =
            case root.highlight of
                HighlightNothing ->
                    HighlightLine id line

                HighlightLine currentID currentLine ->
                    if currentID == id then
                        if currentLine < line then
                            HighlightRange id currentLine line

                        else
                            HighlightRange id line currentLine

                    else
                        HighlightLine id line

                HighlightRange currentID currentLine _ ->
                    if currentID == id then
                        if currentLine < line then
                            HighlightRange id currentLine line

                        else
                            HighlightRange id line currentLine

                    else
                        HighlightLine id line
    in
    ( { root | highlight = hl }, [ ModifyUrl (showHighlight hl) ] )


view :
    { timeZone : Time.Zone, hovered : HoverState.HoverState }
    -> StepTreeModel
    -> Html Message
view session model =
    viewTree session model model.tree 0


assumeStep : StepTreeModel -> StepID -> (Step -> Html Message) -> Html Message
assumeStep model stepId f =
    case Dict.get stepId model.steps of
        Nothing ->
            -- should be impossible
            Html.text ""

        Just step ->
            f step


viewTree :
    { timeZone : Time.Zone, hovered : HoverState.HoverState }
    -> StepTreeModel
    -> StepTree
    -> Int
    -> Html Message
viewTree session model tree depth =
    case tree of
        Task stepId ->
            viewStep model session depth stepId

        Check stepId ->
            viewStep model session depth stepId

        Get stepId ->
            viewStep model session depth stepId

        Run stepId ->
            viewStep model session depth stepId

        Put stepId ->
            viewStep model session depth stepId

        ArtifactInput stepId ->
            viewStep model session depth stepId

        ArtifactOutput stepId ->
            viewStep model session depth stepId

        SetPipeline stepId ->
            viewStep model session depth stepId

        LoadVar stepId ->
            viewStep model session depth stepId

        Try subTree ->
            viewTree session model subTree depth

        Across stepId vars vals substeps ->
            assumeStep model stepId <|
                \step ->
                    viewStepWithBody model session depth step <|
                        (vals
                            |> List.indexedMap
                                (\i vals_ ->
                                    ( vals_
                                    , Dict.get stepId model.steps
                                        |> Maybe.andThen (.expandedHeaders >> Dict.get i)
                                        |> Maybe.withDefault False
                                    , substeps |> Array.get i
                                    )
                                )
                            |> List.filterMap
                                (\( vals_, expanded_, substep ) ->
                                    case substep of
                                        Nothing ->
                                            -- impossible, but need to get rid of the Maybe
                                            Nothing

                                        Just substep_ ->
                                            Just ( vals_, expanded_, substep_ )
                                )
                            |> List.indexedMap
                                (\i ( vals_, expanded_, substep ) ->
                                    let
                                        keyVals =
                                            List.map2 Tuple.pair vars vals_
                                    in
                                    viewAcrossStepSubHeader model session step.id i keyVals expanded_ (depth + 1) substep
                                )
                        )

        Retry stepId steps ->
            assumeStep model stepId <|
                \{ tabFocus } ->
                    let
                        activeTab =
                            case tabFocus of
                                Manual i ->
                                    i

                                Auto ->
                                    Maybe.withDefault 0 (lastActive model steps)
                    in
                    Html.div [ class "retry" ]
                        [ Html.ul
                            (class "retry-tabs" :: Styles.retryTabList)
                            (Array.toList <| Array.indexedMap (viewRetryTab session model stepId activeTab) steps)
                        , case Array.get activeTab steps of
                            Just step ->
                                viewTree session model step depth

                            Nothing ->
                                -- impossible (bogus tab selected)
                                Html.text ""
                        ]

        Timeout subTree ->
            viewTree session model subTree depth

        InParallel trees ->
            Html.div [ class "parallel" ]
                (Array.toList <| Array.map (viewSeq session model depth) trees)

        Do trees ->
            Html.div [ class "do" ]
                (Array.toList <| Array.map (viewSeq session model depth) trees)

        OnSuccess { step, hook } ->
            viewHooked session "success" model depth step hook

        OnFailure { step, hook } ->
            viewHooked session "failure" model depth step hook

        OnAbort { step, hook } ->
            viewHooked session "abort" model depth step hook

        OnError { step, hook } ->
            viewHooked session "error" model depth step hook

        Ensure { step, hook } ->
            viewHooked session "ensure" model depth step hook


viewAcrossStepSubHeader :
    StepTreeModel
    -> { timeZone : Time.Zone, hovered : HoverState.HoverState }
    -> StepID
    -> Int
    -> List ( String, JsonValue )
    -> Bool
    -> Int
    -> StepTree
    -> Html Message
viewAcrossStepSubHeader model session stepID subHeaderIdx keyVals expanded depth subtree =
    let
        state =
            mostSevereStepState model subtree
    in
    Html.div
        [ classList
            [ ( "build-step", True )
            , ( "inactive", not <| isActive state )
            ]
        , style "margin-top" "10px"
        ]
        [ Html.div
            ([ class "header"
             , class "sub-header"
             , onClick <| Click <| StepSubHeader stepID subHeaderIdx
             , style "z-index" <| String.fromInt <| max (maxDepth - depth) 1
             ]
                ++ Styles.stepHeader state
            )
            [ Html.div
                [ style "display" "flex" ]
                [ viewKeyValuePairHeaderLabels keyVals ]
            , Html.div
                [ style "display" "flex" ]
                [ viewStepStateWithoutTooltip state ]
            ]
        , if expanded then
            Html.div
                [ class "step-body"
                , class "clearfix"
                , style "padding-bottom" "0"
                ]
                [ viewTree session model subtree (depth + 1) ]

          else
            Html.text ""
        ]


viewKeyValuePairHeaderLabels : List ( String, JsonValue ) -> Html Message
viewKeyValuePairHeaderLabels keyVals =
    Html.div Styles.keyValuePairHeaderLabel
        (keyVals
            |> List.concatMap
                (\( k, v ) ->
                    flattenJson k v
                        |> List.map
                            (\( key, val ) ->
                                Html.span
                                    [ style "display" "inline-block"
                                    , style "margin-right" "10px"
                                    ]
                                    [ Html.span [ style "color" Colors.pending ]
                                        [ Html.text <| key ++ ": " ]
                                    , Html.text val
                                    ]
                            )
                )
        )


viewRetryTab :
    { r | hovered : HoverState.HoverState }
    -> StepTreeModel
    -> StepID
    -> Int
    -> Int
    -> StepTree
    -> Html Message
viewRetryTab { hovered } model stepId activeTab tab step =
    let
        label =
            String.fromInt (tab + 1)

        active =
            treeIsActive model step

        current =
            activeTab == tab
    in
    Html.li
        ([ classList
            [ ( "current", current )
            , ( "inactive", not active )
            ]
         , onMouseEnter <| Hover <| Just <| StepTab stepId tab
         , onMouseLeave <| Hover Nothing
         , onClick <| Click <| StepTab stepId tab
         ]
            ++ Styles.tab
                { isHovered = HoverState.isHovered (StepTab stepId tab) hovered
                , isCurrent = current
                , isStarted = active
                }
        )
        [ Html.text label ]


viewSeq : { timeZone : Time.Zone, hovered : HoverState.HoverState } -> StepTreeModel -> Int -> StepTree -> Html Message
viewSeq session model depth tree =
    Html.div [ class "seq" ] [ viewTree session model tree depth ]


viewHooked : { timeZone : Time.Zone, hovered : HoverState.HoverState } -> String -> StepTreeModel -> Int -> StepTree -> StepTree -> Html Message
viewHooked session name model depth step hook =
    Html.div [ class "hooked" ]
        [ Html.div [ class "step" ] [ viewTree session model step depth ]
        , Html.div [ class "children" ]
            [ Html.div [ class ("hook hook-" ++ name) ] [ viewTree session model hook depth ]
            ]
        ]


maxDepth : Int
maxDepth =
    10


viewStepWithBody :
    StepTreeModel
    -> { timeZone : Time.Zone, hovered : HoverState.HoverState }
    -> Int
    -> Step
    -> List (Html Message)
    -> Html Message
viewStepWithBody model session depth step body =
    Html.div
        (classList
            [ ( "build-step", True )
            , ( "inactive", not <| isActive step.state )
            ]
            :: (case stepName step.buildStep of
                    Just name ->
                        [ attribute "data-step-name" <| name ]

                    Nothing ->
                        []
               )
        )
        [ Html.div
            ([ class "header"
             , onClick <| Click <| StepHeader step.id
             , style "z-index" <| String.fromInt <| max (maxDepth - depth) 1
             ]
                ++ Styles.stepHeader step.state
            )
            [ viewStepHeader step
            , Html.div
                [ style "display" "flex" ]
                [ viewVersion step model.buildId <| resourceName step.buildStep
                , case Maybe.Extra.or step.imageCheck step.imageGet of
                    Just _ ->
                        viewInitializationToggle step

                    Nothing ->
                        Html.text ""
                , viewStepState step.state (Just step.id)
                ]
            ]
        , if step.initializationExpanded then
            Html.div (class "sub-steps" :: Styles.imageSteps)
                [ case step.imageCheck of
                    Just subTree ->
                        Html.div [ class "seq" ]
                            [ viewTree session model subTree (depth + 1)
                            ]

                    Nothing ->
                        Html.text ""
                , case step.imageGet of
                    Just subTree ->
                        Html.div [ class "seq" ]
                            [ viewTree session model subTree (depth + 1)
                            ]

                    Nothing ->
                        Html.text ""
                ]

          else
            Html.text ""
        , if step.expanded then
            Html.div
                [ class "step-body"
                , class "clearfix"
                ]
                ([ viewMetadata step.metadata
                 , Html.pre [ class "timestamped-logs" ] <|
                    viewLogs step.log step.timestamps model.highlight session.timeZone step.id
                 , case step.error of
                    Nothing ->
                        Html.span [] []

                    Just msg ->
                        Html.span [ class "error" ] [ Html.pre [] [ Html.text msg ] ]
                 ]
                    ++ body
                )

          else
            Html.text ""
        ]


viewInitializationToggle : Step -> Html Message
viewInitializationToggle step =
    let
        domId =
            StepInitialization step.id
    in
    Html.h3
        ([ StrictEvents.onLeftClickStopPropagation (Click domId)
         , onMouseLeave <| Hover Nothing
         , onMouseEnter <| Hover (Just domId)
         , id (toHtmlID domId)
         ]
            ++ Styles.initializationToggle step.initializationExpanded
        )
        [ Icon.icon
            { sizePx = 14
            , image = Assets.CogsIcon
            }
            [ style "background-size" "contain"
            ]
        ]


viewStep : StepTreeModel -> { timeZone : Time.Zone, hovered : HoverState.HoverState } -> Int -> StepID -> Html Message
viewStep model session depth stepId =
    assumeStep model stepId <|
        \step ->
            viewStepWithBody model session depth step []


viewLogs :
    Ansi.Log.Model
    -> Dict Int Time.Posix
    -> Highlight
    -> Time.Zone
    -> String
    -> List (Html Message)
viewLogs { lines } timestamps hl timeZone id =
    Array.toList <|
        Array.indexedMap
            (\idx line ->
                viewTimestampedLine
                    { timestamps = timestamps
                    , highlight = hl
                    , id = id
                    , lineNo = idx + 1
                    , line = line
                    , timeZone = timeZone
                    }
            )
            lines


viewTimestampedLine :
    { timestamps : Dict Int Time.Posix
    , highlight : Highlight
    , id : StepID
    , lineNo : Int
    , line : Ansi.Log.Line
    , timeZone : Time.Zone
    }
    -> Html Message
viewTimestampedLine { timestamps, highlight, id, lineNo, line, timeZone } =
    let
        highlighted =
            case highlight of
                HighlightNothing ->
                    False

                HighlightLine hlId hlLine ->
                    hlId == id && hlLine == lineNo

                HighlightRange hlId hlLine1 hlLine2 ->
                    hlId == id && lineNo >= hlLine1 && lineNo <= hlLine2

        ts =
            Dict.get lineNo timestamps
    in
    Html.tr
        [ classList
            [ ( "timestamped-line", True )
            , ( "highlighted-line", highlighted )
            ]
        , Html.Attributes.id <| id ++ ":" ++ String.fromInt lineNo
        ]
        [ viewTimestamp
            { id = id
            , lineNo = lineNo
            , date = ts
            , timeZone = timeZone
            }
        , viewLine line
        ]


viewLine : Ansi.Log.Line -> Html Message
viewLine line =
    Html.td [ class "timestamped-content" ]
        [ Ansi.Log.viewLine line
        ]


viewTimestamp :
    { id : String
    , lineNo : Int
    , date : Maybe Time.Posix
    , timeZone : Time.Zone
    }
    -> Html Message
viewTimestamp { id, lineNo, date, timeZone } =
    Html.a
        [ href (showHighlight (HighlightLine id lineNo))
        , StrictEvents.onLeftClickOrShiftLeftClick
            (SetHighlight id lineNo)
            (ExtendHighlight id lineNo)
        ]
        [ case date of
            Just d ->
                Html.td
                    [ class "timestamp" ]
                    [ Html.text <|
                        DateFormat.format
                            [ DateFormat.hourMilitaryFixed
                            , DateFormat.text ":"
                            , DateFormat.minuteFixed
                            , DateFormat.text ":"
                            , DateFormat.secondFixed
                            ]
                            timeZone
                            d
                    ]

            _ ->
                Html.td [ class "timestamp placeholder" ] []
        ]


viewVersion :
    Step
    ->
        Maybe
            { r
                | teamName : Concourse.TeamName
                , pipelineName : Concourse.PipelineName
                , pipelineInstanceVars : Concourse.InstanceVars
            }
    -> Maybe String
    -> Html Message
viewVersion step pipelineId name =
    let
        viewVersionContainer =
            case ( step.version, pipelineId, name ) of
                ( Just version, Just pid, Just resource ) ->
                    Html.a
                        [ href <|
                            Routes.toString <|
                                Routes.Resource
                                    { id =
                                        { teamName = pid.teamName
                                        , pipelineName = pid.pipelineName
                                        , pipelineInstanceVars = pid.pipelineInstanceVars
                                        , resourceName = resource
                                        }
                                    , page = Nothing
                                    , version = Just version
                                    , groups = []
                                    }
                        , onMouseLeave <| Hover Nothing
                        , onMouseEnter <| Hover (Just domId)
                        , id (toHtmlID domId)
                        ]

                _ ->
                    Html.div []

        domId =
            StepVersion step.id
    in
    case step.version of
        Just version ->
            viewVersionContainer
                [ DictView.view
                    []
                    (Dict.map (always Html.text) version)
                ]

        _ ->
            Html.text ""


viewMetadata : List MetadataField -> Html Message
viewMetadata meta =
    let
        val value =
            case Url.fromString value of
                Just _ ->
                    Html.a
                        [ href value
                        , target "_blank"
                        , style "text-decoration-line" "underline"
                        ]
                        [ Html.text value ]

                Nothing ->
                    Html.text value

        tr { name, value } =
            Html.tr []
                [ Html.td (Styles.metadataCell Styles.Key)
                    [ Html.text name ]
                , Html.td (Styles.metadataCell Styles.Value)
                    [ val value ]
                ]
    in
    if meta == [] then
        Html.text ""

    else
        meta
            |> List.map tr
            |> Html.table Styles.metadataTable


viewStepStateWithoutTooltip : StepState -> Html Message
viewStepStateWithoutTooltip state =
    viewStepState state Nothing


viewStepState : StepState -> Maybe StepID -> Html Message
viewStepState state stepID =
    let
        attributes =
            style "position" "relative"
                :: (case stepID of
                        Just stepID_ ->
                            [ onMouseEnter <| Hover (Just (StepState stepID_))
                            , onMouseLeave <| Hover Nothing
                            , id <| toHtmlID <| StepState stepID_
                            ]

                        Nothing ->
                            []
                   )
    in
    case state of
        StepStateRunning ->
            Spinner.spinner
                { sizePx = 14
                , margin = "0px"
                }

        StepStatePending ->
            Icon.icon
                { sizePx = 14
                , image = Assets.PendingIcon
                }
                (attribute "data-step-state" "pending"
                    :: Styles.stepStatusIcon
                    ++ attributes
                )

        StepStateInterrupted ->
            Icon.icon
                { sizePx = 14
                , image = Assets.InterruptedIcon
                }
                (attribute "data-step-state" "interrupted"
                    :: Styles.stepStatusIcon
                    ++ attributes
                )

        StepStateCancelled ->
            Icon.icon
                { sizePx = 14
                , image = Assets.CancelledIcon
                }
                (attribute "data-step-state" "cancelled"
                    :: Styles.stepStatusIcon
                    ++ attributes
                )

        StepStateSucceeded ->
            Icon.icon
                { sizePx = 14
                , image = Assets.SuccessCheckIcon
                }
                (attribute "data-step-state" "succeeded"
                    :: Styles.stepStatusIcon
                    ++ attributes
                )

        StepStateFailed ->
            Icon.icon
                { sizePx = 14
                , image = Assets.FailureTimesIcon
                }
                (attribute "data-step-state" "failed"
                    :: Styles.stepStatusIcon
                    ++ attributes
                )

        StepStateErrored ->
            Icon.icon
                { sizePx = 14
                , image = Assets.ExclamationTriangleIcon
                }
                (attribute "data-step-state" "errored"
                    :: Styles.stepStatusIcon
                    ++ attributes
                )


viewStepHeader : Step -> Html Message
viewStepHeader step =
    let
        headerWithContent label changedTooltip content =
            Html.div
                [ style "display" "flex" ]
                [ viewStepHeaderLabel label changedTooltip step.changed step.id
                , Html.h3 [ style "display" "flex" ] content
                ]

        simpleHeader label changedTooltip name =
            headerWithContent label changedTooltip [ Html.text name ]
    in
    case step.buildStep of
        Concourse.BuildStepTask name ->
            simpleHeader "task:" Nothing name

        Concourse.BuildStepSetPipeline name instanceVars ->
            headerWithContent "set_pipeline:" (Just "pipeline config changed") <|
                Html.span [] [ Html.text name ]
                    :: (if Dict.isEmpty instanceVars then
                            []

                        else
                            [ Html.span [ style "margin-left" "10px", style "margin-right" "4px" ] [ Html.text "/" ]
                            , viewKeyValuePairHeaderLabels (Dict.toList instanceVars)
                            ]
                       )

        Concourse.BuildStepLoadVar name ->
            simpleHeader "load_var:" Nothing name

        Concourse.BuildStepCheck name _ ->
            simpleHeader "check:" Nothing name

        Concourse.BuildStepRun message ->
            simpleHeader "run:" Nothing message

        Concourse.BuildStepGet name _ _ _ ->
            simpleHeader "get:" (Just "new version") name

        Concourse.BuildStepPut name _ _ ->
            simpleHeader "put:" Nothing name

        Concourse.BuildStepArtifactInput name ->
            simpleHeader "get:" (Just "new version") name

        Concourse.BuildStepArtifactOutput name ->
            simpleHeader "put:" Nothing name

        Concourse.BuildStepAcross { vars } ->
            simpleHeader "across:" Nothing <| String.join ", " vars

        Concourse.BuildStepDo _ ->
            Html.text ""

        Concourse.BuildStepInParallel _ ->
            Html.text ""

        Concourse.BuildStepOnSuccess _ ->
            Html.text ""

        Concourse.BuildStepOnFailure _ ->
            Html.text ""

        Concourse.BuildStepOnAbort _ ->
            Html.text ""

        Concourse.BuildStepOnError _ ->
            Html.text ""

        Concourse.BuildStepEnsure _ ->
            Html.text ""

        Concourse.BuildStepTry _ ->
            Html.text ""

        Concourse.BuildStepRetry _ ->
            Html.text ""

        Concourse.BuildStepTimeout _ ->
            Html.text ""


stepName : Concourse.BuildStep -> Maybe String
stepName header =
    case header of
        Concourse.BuildStepTask name ->
            Just name

        Concourse.BuildStepSetPipeline name _ ->
            Just name

        Concourse.BuildStepLoadVar name ->
            Just name

        Concourse.BuildStepArtifactInput name ->
            Just name

        Concourse.BuildStepArtifactOutput name ->
            Just name

        Concourse.BuildStepCheck name _ ->
            Just name

        Concourse.BuildStepRun message ->
            Just message

        Concourse.BuildStepGet name _ _ _ ->
            Just name

        Concourse.BuildStepPut name _ _ ->
            Just name

        Concourse.BuildStepAcross { vars } ->
            Just <| String.join ", " vars

        Concourse.BuildStepDo _ ->
            Nothing

        Concourse.BuildStepInParallel _ ->
            Nothing

        Concourse.BuildStepOnSuccess _ ->
            Nothing

        Concourse.BuildStepOnFailure _ ->
            Nothing

        Concourse.BuildStepOnAbort _ ->
            Nothing

        Concourse.BuildStepOnError _ ->
            Nothing

        Concourse.BuildStepEnsure _ ->
            Nothing

        Concourse.BuildStepTry _ ->
            Nothing

        Concourse.BuildStepRetry _ ->
            Nothing

        Concourse.BuildStepTimeout _ ->
            Nothing


resourceName : Concourse.BuildStep -> Maybe String
resourceName step =
    case step of
        Concourse.BuildStepGet _ resource _ _ ->
            resource

        Concourse.BuildStepPut _ resource _ ->
            resource

        _ ->
            Nothing


viewStepHeaderLabel : String -> Maybe String -> Bool -> StepID -> Html Message
viewStepHeaderLabel label changedTooltip changed stepID =
    let
        eventHandlers =
            case ( changedTooltip, changed ) of
                ( Just tooltipMsg, True ) ->
                    [ onMouseLeave <| Hover Nothing
                    , onMouseEnter <| Hover <| Just <| ChangedStepLabel stepID tooltipMsg
                    ]

                _ ->
                    []
    in
    Html.div
        (id (toHtmlID <| ChangedStepLabel stepID "")
            :: Styles.stepHeaderLabel changed
            ++ eventHandlers
        )
        [ Html.text label ]


tooltip : StepTreeModel -> { a | hovered : HoverState.HoverState } -> Maybe Tooltip.Tooltip
tooltip model { hovered } =
    case hovered of
        HoverState.Tooltip (ChangedStepLabel _ text) _ ->
            Just
                { body =
                    Html.div
                        Styles.changedStepTooltip
                        [ Html.text text ]
                , attachPosition =
                    { direction = Tooltip.Top
                    , alignment = Tooltip.Start
                    }
                , arrow = Just 5
                , containerAttrs = Nothing
                }

        HoverState.Tooltip (StepInitialization _) _ ->
            Just
                { body =
                    Html.div
                        Styles.changedStepTooltip
                        [ Html.text "image fetching" ]
                , attachPosition =
                    { direction = Tooltip.Top
                    , alignment = Tooltip.End
                    }
                , arrow = Just 5
                , containerAttrs = Nothing
                }

        HoverState.Tooltip (StepState id) _ ->
            Dict.get id model.steps
                |> Maybe.map stepDurationTooltip

        HoverState.Tooltip (StepVersion _) _ ->
            Just
                { body =
                    Html.div
                        Styles.changedStepTooltip
                        [ Html.text "view in resources page" ]
                , attachPosition =
                    { direction = Tooltip.Top
                    , alignment = Tooltip.Middle 150
                    }
                , arrow = Just 5
                , containerAttrs = Nothing
                }

        _ ->
            Nothing


stepDurationTooltip : Step -> Tooltip.Tooltip
stepDurationTooltip { state, initialize, start, finish } =
    { body =
        Html.div Styles.durationTooltip
            [ case ( initialize, start, finish ) of
                ( Just initializedAt, Just startedAt, Just finishedAt ) ->
                    let
                        initDuration =
                            Duration.between initializedAt startedAt

                        stepDuration =
                            Duration.between startedAt finishedAt
                    in
                    DictView.view []
                        (Dict.fromList
                            [ ( "initialization"
                              , Html.text (Duration.format initDuration)
                              )
                            , ( "step"
                              , Html.text (Duration.format stepDuration)
                              )
                            ]
                        )

                _ ->
                    Html.text (showStepState state)
            ]
    , attachPosition =
        { direction = Tooltip.Top
        , alignment = Tooltip.End
        }
    , arrow = Just 5
    , containerAttrs = Nothing
    }
