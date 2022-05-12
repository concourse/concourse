module Resource.Resource exposing
    ( Flags
    , changeToResource
    , documentTitle
    , getUpdateMessage
    , handleCallback
    , handleDelivery
    , init
    , startingPage
    , subscriptions
    , tooltip
    , update
    , versions
    , view
    , viewPinButton
    , viewVersionBody
    , viewVersionHeader
    )

import Api.Endpoints as Endpoints
import Application.Models exposing (Session)
import Assets
import Build.Output.Models exposing (OutputModel)
import Build.Output.Output
import Build.StepTree.Models as STModels
import Build.StepTree.StepTree as StepTree
import Concourse
import Concourse.BuildStatus
import Concourse.Pagination
    exposing
        ( Direction(..)
        , Page
        , Paginated
        , chevronContainer
        , chevronLeft
        , chevronRight
        )
import DateFormat
import Dict
import Duration
import EffectTransformer exposing (ET)
import HoverState
import Html exposing (Html)
import Html.Attributes
    exposing
        ( attribute
        , class
        , href
        , id
        , readonly
        , style
        , title
        , value
        )
import Html.Events
    exposing
        ( onBlur
        , onClick
        , onFocus
        , onInput
        , onMouseEnter
        , onMouseLeave
        , onMouseOut
        , onMouseOver
        )
import Html.Lazy
import Http
import Keyboard
import List.Extra
import Login.Login as Login
import Maybe.Extra as ME
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects exposing (Effect(..), toHtmlID)
import Message.Message as Message
    exposing
        ( DomID(..)
        , Message(..)
        )
import Message.Subscription as Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Pinned exposing (ResourcePinState(..), VersionPinState(..))
import RemoteData exposing (WebData)
import Resource.Models as Models exposing (Model)
import Resource.Styles
import Routes
import SideBar.SideBar as SideBar exposing (byPipelineId, lookupPipeline)
import StrictEvents
import Svg
import Svg.Attributes as SvgAttributes
import Time
import Tooltip
import UpdateMsg exposing (UpdateMsg)
import UserState exposing (UserState(..))
import Views.DictView as DictView
import Views.Icon as Icon
import Views.Spinner as Spinner
import Views.Styles
import Views.TopBar as TopBar


type alias Flags =
    { resourceId : Concourse.ResourceIdentifier
    , paging : Maybe Concourse.Pagination.Page
    , highlightVersion : Maybe Concourse.Version
    }


pageLimit : Int
pageLimit =
    100


startingPage : Page
startingPage =
    { direction = ToMostRecent, limit = pageLimit }


init : Flags -> ( Model, List Effect )
init flags =
    let
        page =
            flags.paging |> Maybe.withDefault startingPage

        fetchVersionedResource =
            case flags.highlightVersion of
                Just v ->
                    FetchVersionedResourceId flags.resourceId v

                Nothing ->
                    FetchVersionedResources flags.resourceId page

        model =
            { resourceIdentifier = flags.resourceId
            , pageStatus = Err Models.Empty
            , checkStatus = Models.NotChecking
            , lastChecked = Nothing
            , pinnedVersion = NotPinned
            , currentPage = page
            , versions =
                { content = []
                , pagination = { previousPage = Nothing, nextPage = Nothing }
                }
            , now = Nothing
            , pinCommentLoading = False
            , textAreaFocused = False
            , isUserMenuExpanded = False
            , icon = Nothing
            , isEditing = False
            , build = Nothing
            , authorized = True
            , output = Nothing
            , highlight = Routes.HighlightNothing
            , highlightVersion = flags.highlightVersion
            }
    in
    ( model
    , [ FetchResource flags.resourceId
      , fetchVersionedResource
      , GetCurrentTimeZone
      , FetchAllPipelines
      , SyncTextareaHeight ResourceCommentTextarea
      ]
    )


changeToResource : Flags -> ET Model
changeToResource flags ( model, effects ) =
    let
        page =
            flags.paging |> Maybe.withDefault startingPage

        fetchVersionedResource =
            case flags.highlightVersion of
                Just v ->
                    FetchVersionedResourceId model.resourceIdentifier v

                Nothing ->
                    FetchVersionedResources model.resourceIdentifier page
    in
    ( { model
        | currentPage = page
        , versions =
            { content = []
            , pagination = { previousPage = Nothing, nextPage = Nothing }
            }
      }
    , effects
        ++ [ fetchVersionedResource
           , SyncTextareaHeight ResourceCommentTextarea
           ]
    )


initBuild : Maybe Concourse.Build -> ET Model
initBuild mbuild ( model, effects ) =
    case mbuild of
        Nothing ->
            ( model, effects )

        Just build ->
            if Maybe.map .id model.build == Just build.id then
                ( { model | build = Just build }, effects )

            else
                let
                    ( output, outputCmd ) =
                        Build.Output.Output.init model.highlight build
                in
                ( { model | build = Just build, output = Just output }
                , effects ++ CloseBuildEventStream :: outputCmd
                )


updatePinnedVersion : Concourse.Resource -> Model -> Model
updatePinnedVersion resource model =
    case ( resource.pinnedVersion, resource.pinnedInConfig ) of
        ( Nothing, _ ) ->
            case model.pinnedVersion of
                PinningTo _ ->
                    model

                _ ->
                    { model | pinnedVersion = NotPinned }

        ( Just v, True ) ->
            { model | pinnedVersion = PinnedStaticallyTo v }

        ( Just newVersion, False ) ->
            let
                pristineComment =
                    resource.pinComment |> Maybe.withDefault ""
            in
            case model.pinnedVersion of
                UnpinningFrom c _ ->
                    { model | pinnedVersion = UnpinningFrom c newVersion }

                PinnedDynamicallyTo { comment } _ ->
                    { model
                        | pinnedVersion =
                            PinnedDynamicallyTo
                                { comment = comment
                                , pristineComment = pristineComment
                                }
                                newVersion
                    }

                Switching _ v _ ->
                    if v == newVersion then
                        model

                    else
                        { model
                            | pinnedVersion =
                                PinnedDynamicallyTo
                                    { comment = pristineComment
                                    , pristineComment = pristineComment
                                    }
                                    newVersion
                        }

                _ ->
                    { model
                        | pinnedVersion =
                            PinnedDynamicallyTo
                                { comment = pristineComment
                                , pristineComment = pristineComment
                                }
                                newVersion
                    }


getUpdateMessage : Model -> UpdateMsg
getUpdateMessage model =
    if model.pageStatus == Err Models.NotFound then
        UpdateMsg.NotFound

    else
        UpdateMsg.AOK


subscriptions : Model -> List Subscription
subscriptions model =
    let
        buildEventsUrl =
            model.output
                |> Maybe.andThen .eventStreamUrlPath
    in
    [ OnClockTick Subscription.FiveSeconds
    , OnClockTick Subscription.OneSecond
    , OnKeyDown
    , OnKeyUp
    , OnWindowResize
    ]
        ++ (case buildEventsUrl of
                Nothing ->
                    []

                Just url ->
                    [ Subscription.FromEventSource ( url, [ "end", "event" ] ) ]
           )


handleCallback : Callback -> Session -> ET Model
handleCallback callback session ( model, effects ) =
    case callback of
        ResourceFetched (Ok resource) ->
            ( { model
                | pageStatus = Ok ()
                , resourceIdentifier =
                    { teamName = resource.teamName
                    , pipelineName = resource.pipelineName
                    , pipelineInstanceVars = resource.pipelineInstanceVars
                    , resourceName = resource.name
                    }
                , checkStatus =
                    case resource.build of
                        Nothing ->
                            Models.NotChecking

                        Just { id, status } ->
                            case status of
                                Concourse.BuildStatus.BuildStatusSucceeded ->
                                    Models.NotChecking

                                Concourse.BuildStatus.BuildStatusStarted ->
                                    Models.CurrentlyChecking id

                                _ ->
                                    Models.NotChecking
                , lastChecked = resource.lastChecked
                , icon = resource.icon
              }
                |> updatePinnedVersion resource
            , effects
                ++ (case resource.icon of
                        Just icon ->
                            [ RenderSvgIcon <| icon ]

                        Nothing ->
                            []
                   )
                ++ [ SyncTextareaHeight ResourceCommentTextarea ]
            )
                |> initBuild resource.build

        ResourceFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, effects ++ [ RedirectToLogin ] )

                    else if status.code == 404 then
                        ( { model | pageStatus = Err Models.NotFound }, effects )

                    else
                        ( model, effects )

                _ ->
                    ( model, effects )

        VersionedResourceIdFetched (Ok versionedResource) ->
            let
                page =
                    case versionedResource of
                        Just vr ->
                            { direction = To vr.id
                            , limit = model.currentPage.limit
                            }

                        Nothing ->
                            startingPage

                vri vr =
                    Concourse.toVersionedResourceId model.resourceIdentifier vr
            in
            ( { model | currentPage = page }
            , effects
                ++ (case versionedResource of
                        Just vr ->
                            [ FetchInputTo <| vri vr
                            , FetchOutputOf <| vri vr
                            ]

                        Nothing ->
                            []
                   )
                ++ [ FetchVersionedResources model.resourceIdentifier page
                   ]
            )

        VersionedResourcesFetched (Ok ( requestedPage, paginated )) ->
            let
                resourceVersions =
                    { pagination = paginated.pagination
                    , content =
                        paginated.content
                            |> List.map
                                (\vr ->
                                    let
                                        existingVersion : Maybe Models.Version
                                        existingVersion =
                                            model.versions.content
                                                |> List.Extra.find
                                                    (\v ->
                                                        v.id.versionID == vr.id
                                                    )

                                        enabledStateAccordingToServer : Models.VersionEnabledState
                                        enabledStateAccordingToServer =
                                            if vr.enabled then
                                                Models.Enabled

                                            else
                                                Models.Disabled

                                        expanded =
                                            case model.highlightVersion of
                                                Just v ->
                                                    vr.version == v

                                                Nothing ->
                                                    False
                                    in
                                    case existingVersion of
                                        Just ev ->
                                            { ev
                                                | enabled =
                                                    if ev.enabled == Models.Changing then
                                                        Models.Changing

                                                    else
                                                        enabledStateAccordingToServer
                                            }

                                        Nothing ->
                                            { id =
                                                { teamName = model.resourceIdentifier.teamName
                                                , pipelineName = model.resourceIdentifier.pipelineName
                                                , pipelineInstanceVars = model.resourceIdentifier.pipelineInstanceVars
                                                , resourceName = model.resourceIdentifier.resourceName
                                                , versionID = vr.id
                                                }
                                            , version = vr.version
                                            , metadata = vr.metadata
                                            , enabled = enabledStateAccordingToServer
                                            , expanded = expanded
                                            , inputTo = []
                                            , outputOf = []
                                            }
                                )
                    }

                newModel =
                    \newPage newEffects ->
                        ( { model
                            | versions = resourceVersions
                            , currentPage = newPage
                          }
                        , newEffects
                        )
            in
            if
                Concourse.Pagination.isPreviousPage requestedPage
                    && (List.length resourceVersions.content < pageLimit)
            then
                -- otherwise a new version would show up as a single element page
                newModel startingPage <|
                    effects
                        ++ [ FetchVersionedResources model.resourceIdentifier startingPage
                           , NavigateTo <|
                                Routes.toString <|
                                    Routes.Resource
                                        { id = model.resourceIdentifier
                                        , page = Just startingPage
                                        , version = Nothing
                                        , groups = []
                                        }
                           ]

            else if Concourse.Pagination.equal model.currentPage requestedPage then
                newModel requestedPage effects

            else
                ( model, effects )

        InputToFetched (Ok ( versionID, builds )) ->
            ( updateVersion versionID (\v -> { v | inputTo = builds }) model
            , effects
            )

        OutputOfFetched (Ok ( versionID, builds )) ->
            ( updateVersion versionID (\v -> { v | outputOf = builds }) model
            , effects
            )

        VersionPinned (Ok ()) ->
            case ( session.userState, model.now ) of
                ( UserStateLoggedIn user, Just time ) ->
                    let
                        pinningTo =
                            case model.pinnedVersion of
                                PinningTo pt ->
                                    Just pt

                                Switching _ _ pt ->
                                    Just pt

                                _ ->
                                    Nothing

                        commentText =
                            "pinned by "
                                ++ Login.userDisplayName user
                                ++ " at "
                                ++ formatDate session.timeZone time
                    in
                    ( { model
                        | pinnedVersion =
                            model.versions.content
                                |> List.Extra.find (\v -> Just v.id == pinningTo)
                                |> Maybe.map .version
                                |> Maybe.map
                                    (PinnedDynamicallyTo
                                        { comment = commentText
                                        , pristineComment = ""
                                        }
                                    )
                                |> Maybe.withDefault NotPinned
                      }
                    , effects
                        ++ [ SetPinComment
                                model.resourceIdentifier
                                commentText
                           ]
                    )

                _ ->
                    ( model, effects )

        VersionPinned (Err _) ->
            ( { model | pinnedVersion = NotPinned }
            , effects
            )

        VersionUnpinned (Ok ()) ->
            ( { model | pinnedVersion = NotPinned }
            , effects ++ [ FetchResource model.resourceIdentifier ]
            )

        VersionUnpinned (Err _) ->
            ( { model | pinnedVersion = Pinned.quitUnpinning model.pinnedVersion }
            , effects
            )

        VersionToggled action versionID result ->
            let
                newEnabledState : Models.VersionEnabledState
                newEnabledState =
                    case ( result, action ) of
                        ( Ok (), Message.Enable ) ->
                            Models.Enabled

                        ( Ok (), Message.Disable ) ->
                            Models.Disabled

                        ( Err _, Message.Enable ) ->
                            Models.Disabled

                        ( Err _, Message.Disable ) ->
                            Models.Enabled
            in
            ( updateVersion versionID (\v -> { v | enabled = newEnabledState }) model
            , effects
            )

        Checked (Ok _) ->
            ( model, effects ++ [ FetchResource model.resourceIdentifier ] )

        Checked (Err (Http.BadStatus { status })) ->
            ( model
            , if status.code == 401 then
                effects ++ [ RedirectToLogin ]

              else
                effects
            )

        CommentSet result ->
            ( { model
                | pinCommentLoading = False
                , pinnedVersion =
                    case ( result, model.pinnedVersion ) of
                        ( Ok (), PinnedDynamicallyTo { comment } v ) ->
                            PinnedDynamicallyTo
                                { comment = comment
                                , pristineComment = comment
                                }
                                v

                        ( _, pv ) ->
                            pv
                , isEditing = result /= Ok ()
              }
            , effects
                ++ [ FetchResource model.resourceIdentifier
                   , SyncTextareaHeight ResourceCommentTextarea
                   ]
            )

        PlanAndResourcesFetched buildId (Ok planAndResources) ->
            updateOutput
                (Build.Output.Output.planAndResourcesFetched
                    buildId
                    planAndResources
                )
                ( model
                , effects
                    ++ [ Effects.OpenBuildEventStream
                            { url =
                                Endpoints.BuildEventStream
                                    |> Endpoints.Build buildId
                                    |> Endpoints.toString []
                            , eventTypes = [ "end", "event" ]
                            }
                       ]
                )

        PlanAndResourcesFetched _ (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 || status.code == 403 then
                        ( { model | authorized = False }, effects )

                    else
                        ( model, effects )

                _ ->
                    ( model, effects )

        _ ->
            ( model, effects )


handleDelivery : { a | hovered : HoverState.HoverState } -> Delivery -> ET Model
handleDelivery session delivery ( model, effects ) =
    (case delivery of
        KeyDown keyEvent ->
            if
                (keyEvent.code == Keyboard.Enter)
                    && Keyboard.hasControlModifier keyEvent
                    && model.textAreaFocused
            then
                ( model
                , case model.pinnedVersion of
                    PinnedDynamicallyTo { comment } _ ->
                        effects ++ [ SetPinComment model.resourceIdentifier comment ]

                    _ ->
                        effects
                )

            else
                ( model, effects )

        ClockTicked OneSecond time ->
            ( { model | now = Just time }
            , case session.hovered of
                HoverState.Hovered (StepState stepID) ->
                    [ GetViewportOf (StepState stepID) ]

                _ ->
                    []
            )

        ClockTicked FiveSeconds _ ->
            ( model
            , effects
                ++ [ FetchResource model.resourceIdentifier
                   , FetchVersionedResources model.resourceIdentifier model.currentPage
                   , FetchAllPipelines
                   ]
                ++ fetchDataForExpandedVersions model
            )

        WindowResized _ _ ->
            ( model
            , effects ++ [ SyncTextareaHeight ResourceCommentTextarea ]
            )

        EventsReceived (Ok envelopes) ->
            let
                ended =
                    List.any (\{ data } -> data == STModels.End) envelopes
            in
            updateOutput
                (Build.Output.Output.handleEnvelopes envelopes)
                ( model
                , effects
                    ++ (if ended then
                            [ FetchResource model.resourceIdentifier
                            , FetchVersionedResources model.resourceIdentifier model.currentPage
                            ]

                        else
                            []
                       )
                )

        _ ->
            ( model, effects )
    )
        |> Tooltip.handleDelivery session delivery


update : Message -> ET Model
update msg ( model, effects ) =
    case msg of
        Click (PaginationButton page) ->
            ( { model | currentPage = page }
            , effects
                ++ [ FetchVersionedResources model.resourceIdentifier <| page
                   , NavigateTo <|
                        Routes.toString <|
                            Routes.Resource
                                { id = model.resourceIdentifier
                                , page = Just page
                                , version = Nothing
                                , groups = []
                                }
                   ]
            )

        Click (VersionHeader versionID) ->
            let
                version : Maybe Models.Version
                version =
                    model.versions.content
                        |> List.Extra.find (.id >> (==) versionID)

                newExpandedState : Bool
                newExpandedState =
                    case version of
                        Just v ->
                            not v.expanded

                        Nothing ->
                            False
            in
            ( updateVersion
                versionID
                (\v ->
                    { v | expanded = newExpandedState }
                )
                model
            , if newExpandedState then
                effects
                    ++ [ FetchInputTo versionID
                       , FetchOutputOf versionID
                       ]

              else
                effects
            )

        Click (PinButton versionID) ->
            let
                version : Maybe Models.Version
                version =
                    model.versions.content
                        |> List.Extra.find (\v -> v.id == versionID)
            in
            case model.pinnedVersion of
                PinnedDynamicallyTo _ v ->
                    version
                        |> Maybe.map
                            (\vn ->
                                if vn.version == v then
                                    ( { model
                                        | pinnedVersion =
                                            Pinned.startUnpinning model.pinnedVersion
                                      }
                                    , effects
                                        ++ [ DoUnpinVersion model.resourceIdentifier ]
                                    )

                                else
                                    ( { model
                                        | pinnedVersion =
                                            Pinned.startPinningTo versionID model.pinnedVersion
                                      }
                                    , effects
                                        ++ [ DoPinVersion vn.id ]
                                    )
                            )
                        |> Maybe.withDefault ( model, effects )

                NotPinned ->
                    ( { model
                        | pinnedVersion =
                            Pinned.startPinningTo versionID model.pinnedVersion
                      }
                    , case version of
                        Just _ ->
                            effects ++ [ DoPinVersion versionID ]

                        Nothing ->
                            effects
                    )

                _ ->
                    ( model, effects )

        Click PinIcon ->
            case model.pinnedVersion of
                PinnedDynamicallyTo _ _ ->
                    ( { model
                        | pinnedVersion =
                            Pinned.startUnpinning model.pinnedVersion
                      }
                    , effects ++ [ DoUnpinVersion model.resourceIdentifier ]
                    )

                _ ->
                    ( model, effects )

        Click (VersionToggle versionID) ->
            let
                enabledState : Maybe Models.VersionEnabledState
                enabledState =
                    model.versions.content
                        |> List.Extra.find (.id >> (==) versionID)
                        |> Maybe.map .enabled
            in
            case enabledState of
                Just Models.Enabled ->
                    ( updateVersion versionID
                        (\v ->
                            { v | enabled = Models.Changing }
                        )
                        model
                    , effects ++ [ DoToggleVersion Message.Disable versionID ]
                    )

                Just Models.Disabled ->
                    ( updateVersion versionID
                        (\v ->
                            { v | enabled = Models.Changing }
                        )
                        model
                    , effects ++ [ DoToggleVersion Message.Enable versionID ]
                    )

                _ ->
                    ( model, effects )

        Click (CheckButton isAuthorized) ->
            if isAuthorized then
                ( { model | checkStatus = Models.CheckPending }
                , effects ++ [ DoCheck model.resourceIdentifier ]
                )

            else
                ( model, effects ++ [ RedirectToLogin ] )

        Click EditButton ->
            ( { model | isEditing = True }
            , effects ++ [ Focus (toHtmlID ResourceCommentTextarea) ]
            )

        Click (StepHeader id) ->
            updateOutput
                (Build.Output.Output.handleStepTreeMsg <| StepTree.toggleStep id)
                ( model, effects )

        Click (StepInitialization id) ->
            updateOutput
                (Build.Output.Output.handleStepTreeMsg <| StepTree.toggleStepInitialization id)
                ( model, effects ++ [ SyncStickyBuildLogHeaders ] )

        EditComment input ->
            let
                newPinnedVersion =
                    case model.pinnedVersion of
                        PinnedDynamicallyTo { pristineComment } v ->
                            PinnedDynamicallyTo
                                { comment = input
                                , pristineComment = pristineComment
                                }
                                v

                        x ->
                            x
            in
            ( { model | pinnedVersion = newPinnedVersion }
            , effects ++ [ SyncTextareaHeight ResourceCommentTextarea ]
            )

        Click SaveCommentButton ->
            case model.pinnedVersion of
                PinnedDynamicallyTo commentState _ ->
                    let
                        commentChanged =
                            commentState.comment /= commentState.pristineComment
                    in
                    if commentChanged then
                        ( { model | pinCommentLoading = True }
                        , effects
                            ++ [ SetPinComment
                                    model.resourceIdentifier
                                    commentState.comment
                               ]
                        )

                    else
                        ( model, effects )

                _ ->
                    ( model, effects )

        FocusTextArea ->
            ( { model | textAreaFocused = True }, effects )

        BlurTextArea ->
            ( { model | textAreaFocused = False }, effects )

        _ ->
            ( model, effects )


updateVersion :
    Models.VersionId
    -> (Models.Version -> Models.Version)
    -> Model
    -> Model
updateVersion versionID updateFunc model =
    let
        newVersionsContent : List Models.Version
        newVersionsContent =
            model.versions.content
                |> List.Extra.updateIf (.id >> (==) versionID) updateFunc

        resourceVersions : Paginated Models.Version
        resourceVersions =
            model.versions
    in
    { model | versions = { resourceVersions | content = newVersionsContent } }


documentTitle : Model -> String
documentTitle model =
    model.resourceIdentifier.resourceName


type alias VersionPresenter =
    { id : Models.VersionId
    , version : Concourse.Version
    , metadata : Concourse.Metadata
    , enabled : Models.VersionEnabledState
    , expanded : Bool
    , inputTo : List Concourse.Build
    , outputOf : List Concourse.Build
    , pinState : VersionPinState
    }


versions :
    { a
        | versions : Paginated Models.Version
        , pinnedVersion : Models.PinnedVersion
    }
    -> List VersionPresenter
versions model =
    model.versions.content
        |> List.map
            (\v ->
                { id = v.id
                , version = v.version
                , metadata = v.metadata
                , enabled = v.enabled
                , expanded = v.expanded
                , inputTo = v.inputTo
                , outputOf = v.outputOf
                , pinState = Pinned.pinState v.version v.id model.pinnedVersion
                }
            )


view : Session -> Model -> Html Message
view session model =
    let
        route =
            Routes.Resource
                { id = model.resourceIdentifier
                , page = Nothing
                , version = Nothing
                , groups = Routes.getGroups session.route
                }
    in
    Html.div
        (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
        [ Html.div
            (id "top-bar-app" :: Views.Styles.topBar False)
            (SideBar.sideBarIcon session
                :: TopBar.breadcrumbs session route
                ++ [ Login.view session.userState model ]
            )
        , Html.div
            (id "page-below-top-bar" :: Views.Styles.pageBelowTopBar route)
            [ SideBar.view session
                (Just
                    { pipelineName = model.resourceIdentifier.pipelineName
                    , pipelineInstanceVars = model.resourceIdentifier.pipelineInstanceVars
                    , teamName = model.resourceIdentifier.teamName
                    }
                )
            , if model.pageStatus == Err Models.Empty then
                Html.text ""

              else
                Html.div
                    [ style "flex-grow" "1"
                    , style "display" "flex"
                    , style "flex-direction" "column"
                    ]
                    [ header session model
                    , body session model
                    ]
            ]
        ]


tooltip : Model -> Session -> Maybe Tooltip.Tooltip
tooltip model session =
    case session.hovered of
        HoverState.Tooltip (CheckButton _) _ ->
            Just
                { body = Html.text "trigger manual check"
                , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.Start }
                , arrow = Just 5
                , containerAttrs = Nothing
                }

        HoverState.Tooltip PinIcon _ ->
            Just
                { body = Html.text "unpin version"
                , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.Start }
                , arrow = Just 5
                , containerAttrs = Nothing
                }

        HoverState.Tooltip EditButton _ ->
            Just
                { body = Html.text "edit pin comment"
                , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.End }
                , arrow = Just 5
                , containerAttrs = Nothing
                }

        HoverState.Tooltip (PinButton id) _ ->
            let
                version =
                    model.versions.content |> List.Extra.find (\v -> v.id == id)

                isPinnedDynamically =
                    case ( version, model.pinnedVersion ) of
                        ( Just cur, PinnedDynamicallyTo _ v ) ->
                            cur.version == v

                        _ ->
                            False

                isStatic =
                    case model.pinnedVersion of
                        PinnedStaticallyTo _ ->
                            True

                        _ ->
                            False
            in
            Just
                { body =
                    Html.text <|
                        if isStatic then
                            "version is pinned in the pipeline config"

                        else if isPinnedDynamically then
                            "unpin version"

                        else
                            "pin version"
                , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.Start }
                , arrow = Just 5
                , containerAttrs = Nothing
                }

        HoverState.Tooltip (VersionToggle id) _ ->
            let
                enabled =
                    model
                        |> versions
                        |> List.Extra.find (\v -> v.id == id)
                        |> Maybe.map .enabled
            in
            (case enabled of
                Just Models.Enabled ->
                    Just "disable version"

                Just Models.Disabled ->
                    Just "enable version"

                _ ->
                    Nothing
            )
                |> Maybe.map
                    (\text ->
                        { body =
                            Html.text text
                        , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.Start }
                        , arrow = Just 5
                        , containerAttrs = Nothing
                        }
                    )

        HoverState.Tooltip PinBar _ ->
            case model.pinnedVersion of
                PinnedStaticallyTo _ ->
                    Just
                        { body = Html.text "version is pinned in the pipeline config"
                        , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.Start }
                        , arrow = Nothing
                        , containerAttrs = Nothing
                        }

                _ ->
                    Nothing

        HoverState.Tooltip NextPageButton _ ->
            Just
                { body = Html.text "view next page"
                , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.End }
                , arrow = Just 5
                , containerAttrs = Nothing
                }

        HoverState.Tooltip PreviousPageButton _ ->
            Just
                { body = Html.text "view previous page"
                , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.End }
                , arrow = Just 5
                , containerAttrs = Nothing
                }

        HoverState.Tooltip (InputsTo _) _ ->
            Just
                { body = Html.text "view all downstream builds and resources"
                , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.End }
                , arrow = Just 5
                , containerAttrs = Nothing
                }

        HoverState.Tooltip (OutputsOf _) _ ->
            Just
                { body = Html.text "view all upstream builds and resources"
                , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.End }
                , arrow = Just 5
                , containerAttrs = Nothing
                }

        HoverState.Tooltip (UserDisplayName username) _ ->
            Login.tooltip username

        _ ->
            model.output
                |> Maybe.andThen .steps
                |> Maybe.andThen (\steps -> StepTree.tooltip steps session)


header : Session -> Model -> Html Message
header session model =
    let
        archived =
            isPipelineArchived session model.resourceIdentifier

        lastCheckedView =
            case ( model.now, model.lastChecked, archived ) of
                ( Just now, Just date, False ) ->
                    viewLastChecked session.timeZone now date

                ( _, _, _ ) ->
                    Html.text ""

        iconView =
            case model.icon of
                Just icon ->
                    Svg.svg
                        [ style "height" "24px"
                        , style "width" "24px"
                        , style "margin-left" "-6px"
                        , style "margin-right" "10px"
                        , SvgAttributes.fill "white"
                        ]
                        [ Svg.use [ SvgAttributes.xlinkHref ("#" ++ icon ++ "-svg-icon") ] []
                        ]

                Nothing ->
                    Html.text ""
    in
    Html.div
        (id "page-header" :: Resource.Styles.headerBar)
        [ Html.h1
            Resource.Styles.headerResourceName
            [ iconView
            , Html.text model.resourceIdentifier.resourceName
            ]
        , Html.div
            Resource.Styles.headerLastCheckedSection
            [ lastCheckedView ]
        , paginationMenu session model
        ]


body :
    { a
        | userState : UserState
        , pipelines : WebData (List Concourse.Pipeline)
        , hovered : HoverState.HoverState
        , timeZone : Time.Zone
        , featureFlags : Concourse.FeatureFlags
    }
    -> Model
    -> Html Message
body session model =
    let
        sectionModel =
            { checkStatus = model.checkStatus
            , build = model.build
            , hovered = session.hovered
            , userState = session.userState
            , timeZone = session.timeZone
            , teamName = model.resourceIdentifier.teamName
            , authorized = model.authorized
            , output = model.output
            }

        archived =
            isPipelineArchived session model.resourceIdentifier
    in
    Html.div
        (id "body" :: Resource.Styles.body)
    <|
        (if model.pinnedVersion == NotPinned then
            if archived then
                []

            else
                [ checkSection sectionModel ]

         else
            [ pinTools session model ]
        )
            ++ [ viewVersionedResources session model ]


paginationMenu :
    { a | hovered : HoverState.HoverState }
    ->
        { b
            | versions : Paginated Models.Version
            , resourceIdentifier : Concourse.ResourceIdentifier
        }
    -> Html Message
paginationMenu { hovered } model =
    let
        previousButtonEventHandler =
            case model.versions.pagination.previousPage of
                Nothing ->
                    []

                Just pp ->
                    [ onClick <| Click <| PaginationButton pp ]

        nextButtonEventHandler =
            case model.versions.pagination.nextPage of
                Nothing ->
                    []

                Just np ->
                    let
                        updatedPage =
                            { np | limit = 100 }
                    in
                    [ onClick <| Click <| PaginationButton updatedPage ]
    in
    Html.div
        (id "pagination" :: Resource.Styles.pagination)
        [ case model.versions.pagination.previousPage of
            Nothing ->
                Html.div
                    chevronContainer
                    [ Html.div
                        (chevronLeft
                            { enabled = False
                            , hovered = False
                            }
                        )
                        []
                    ]

            Just page ->
                Html.div
                    ([ onMouseEnter <| Hover <| Just Message.PreviousPageButton
                     , onMouseLeave <| Hover Nothing
                     ]
                        ++ chevronContainer
                        ++ previousButtonEventHandler
                    )
                    [ Html.a
                        ([ href <|
                            Routes.toString <|
                                Routes.Resource
                                    { id = model.resourceIdentifier
                                    , page = Just page
                                    , version = Nothing
                                    , groups = []
                                    }
                         , attribute "aria-label" "Previous Page"
                         , id <| toHtmlID PreviousPageButton
                         ]
                            ++ chevronLeft
                                { enabled = True
                                , hovered = HoverState.isHovered PreviousPageButton hovered
                                }
                        )
                        []
                    ]
        , case model.versions.pagination.nextPage of
            Nothing ->
                Html.div
                    chevronContainer
                    [ Html.div
                        (chevronRight
                            { enabled = False
                            , hovered = False
                            }
                        )
                        []
                    ]

            Just page ->
                Html.div
                    ([ onMouseEnter <| Hover <| Just Message.NextPageButton
                     , onMouseLeave <| Hover Nothing
                     ]
                        ++ chevronContainer
                        ++ nextButtonEventHandler
                    )
                    [ Html.a
                        ([ href <|
                            Routes.toString <|
                                Routes.Resource
                                    { id = model.resourceIdentifier
                                    , page = Just page
                                    , version = Nothing
                                    , groups = []
                                    }
                         , attribute "aria-label" "Next Page"
                         , id <| toHtmlID NextPageButton
                         ]
                            ++ chevronRight
                                { enabled = True
                                , hovered = HoverState.isHovered NextPageButton hovered
                                }
                        )
                        []
                    ]
        ]


checkSection :
    { a
        | checkStatus : Models.CheckStatus
        , build : Maybe Concourse.Build
        , hovered : HoverState.HoverState
        , userState : UserState
        , teamName : String
        , timeZone : Time.Zone
        , authorized : Bool
        , output : Maybe OutputModel
    }
    -> Html Message
checkSection ({ checkStatus, build } as model) =
    let
        spinner =
            Spinner.spinner
                { sizePx = 14
                , margin = "7px"
                }

        icon image =
            Icon.icon
                { sizePx = 28
                , image = image
                }
                Resource.Styles.checkStatusIcon

        ( checkMessage, statusIcon ) =
            case Maybe.map .status build of
                Nothing ->
                    ( "not checked yet", icon Assets.PendingIcon )

                Just Concourse.BuildStatus.BuildStatusFailed ->
                    ( "check failed", icon Assets.FailureTimesIcon )

                Just Concourse.BuildStatus.BuildStatusPending ->
                    ( "check pending", icon Assets.PendingIcon )

                Just Concourse.BuildStatus.BuildStatusStarted ->
                    ( "check in progress", spinner )

                Just Concourse.BuildStatus.BuildStatusSucceeded ->
                    ( "check succeeded", icon Assets.SuccessCheckIcon )

                Just Concourse.BuildStatus.BuildStatusErrored ->
                    ( "check errored", icon Assets.ExclamationTriangleIcon )

                Just Concourse.BuildStatus.BuildStatusAborted ->
                    ( "check aborted", icon Assets.InterruptedIcon )

        statusBar =
            Html.div
                (class "resource-check-status-summary" :: Resource.Styles.checkBarStatus)
                [ Html.h3 [] [ Html.text checkMessage ]
                , statusIcon
                ]

        checkBar =
            Html.div
                [ style "display" "flex" ]
                [ checkButton model
                , Html.div Resource.Styles.checkStatus
                    [ if model.authorized && model.output /= Nothing then
                        Html.Lazy.lazy3
                            viewBuildOutput
                            model.timeZone
                            (Build.Output.Output.filterHoverState model.hovered)
                            model.output

                      else
                        statusBar
                    ]
                ]
    in
    Html.div [ class "resource-check-status" ] [ checkBar ]


checkButton :
    { a
        | hovered : HoverState.HoverState
        , userState : UserState
        , teamName : String
        , checkStatus : Models.CheckStatus
    }
    -> Html Message
checkButton ({ hovered, userState, checkStatus } as params) =
    let
        isMember =
            UserState.isMember params

        isHovered =
            HoverState.isHovered (CheckButton isMember) hovered

        isCurrentlyChecking =
            case checkStatus of
                Models.CheckPending ->
                    True

                Models.CurrentlyChecking _ ->
                    True

                _ ->
                    False

        isAnonymous =
            UserState.isAnonymous userState

        isClickable =
            (isAnonymous || isMember)
                && not isCurrentlyChecking

        isHighlighted =
            (isClickable && isHovered) || isCurrentlyChecking
    in
    Html.div
        ([ id <| toHtmlID <| CheckButton isMember
         , onMouseEnter <| Hover <| Just <| CheckButton isMember
         , onMouseLeave <| Hover Nothing
         ]
            ++ Resource.Styles.checkButton isClickable
            ++ (if isClickable then
                    [ onClick <| Click <| CheckButton isMember ]

                else
                    []
               )
        )
        [ Icon.icon
            { sizePx = 20
            , image = Assets.RefreshIcon
            }
            (Resource.Styles.checkButtonIcon isHighlighted)
        ]


commentBar :
    { a
        | userState : UserState
        , pipelines : WebData (List Concourse.Pipeline)
        , hovered : HoverState.HoverState
    }
    ->
        { b
            | pinnedVersion : Models.PinnedVersion
            , resourceIdentifier : Concourse.ResourceIdentifier
            , pinCommentLoading : Bool
            , isEditing : Bool
        }
    -> Html Message
commentBar session { resourceIdentifier, pinnedVersion, pinCommentLoading, isEditing } =
    case pinnedVersion of
        PinnedDynamicallyTo commentState _ ->
            Html.div
                (id "comment-bar" :: Resource.Styles.commentBar True)
                [ Html.div
                    (id "icon-container" :: Resource.Styles.commentBarIconContainer isEditing)
                    (Icon.icon
                        { sizePx = 16
                        , image = Assets.MessageIcon
                        }
                        Resource.Styles.commentBarMessageIcon
                        :: (if
                                UserState.isMember
                                    { teamName = resourceIdentifier.teamName
                                    , userState = session.userState
                                    }
                                    && not (isPipelineArchived session resourceIdentifier)
                            then
                                [ Html.textarea
                                    ([ id (toHtmlID ResourceCommentTextarea)
                                     , value commentState.comment
                                     , onInput EditComment
                                     , onFocus FocusTextArea
                                     , onBlur BlurTextArea
                                     , readonly (not isEditing)
                                     ]
                                        ++ Resource.Styles.commentTextArea
                                    )
                                    []
                                , Html.div (id "edit-save-wrapper" :: Resource.Styles.editSaveWrapper)
                                    (if isEditing == False then
                                        [ editButton session ]

                                     else
                                        [ saveButton commentState pinCommentLoading session.hovered ]
                                    )
                                ]

                            else
                                [ Html.pre
                                    Resource.Styles.commentText
                                    [ Html.text commentState.pristineComment ]
                                ]
                           )
                    )
                ]

        _ ->
            Html.text ""


editButton : { a | hovered : HoverState.HoverState } -> Html Message
editButton session =
    Icon.icon
        { sizePx = 16
        , image = Assets.PencilIcon
        }
        ([ id <| toHtmlID EditButton
         , onMouseEnter <| Hover <| Just EditButton
         , onMouseLeave <| Hover Nothing
         , onClick <| Click EditButton
         ]
            ++ Resource.Styles.editButton (HoverState.isHovered EditButton session.hovered)
        )


saveButton :
    { s | comment : String, pristineComment : String }
    -> Bool
    -> HoverState.HoverState
    -> Html Message
saveButton commentState pinCommentLoading hovered =
    Html.button
        (let
            commentChanged =
                commentState.comment
                    /= commentState.pristineComment
         in
         [ id "save-button"
         , onMouseEnter <| Hover <| Just SaveCommentButton
         , onMouseLeave <| Hover Nothing
         , onClick <| Click SaveCommentButton
         ]
            ++ Resource.Styles.commentSaveButton
                { isHovered = HoverState.isHovered SaveCommentButton hovered
                , commentChanged = commentChanged
                , pinCommentLoading = pinCommentLoading
                }
        )
        (if pinCommentLoading then
            [ Spinner.spinner
                { sizePx = 12
                , margin = "0"
                }
            ]

         else
            [ Html.text "save" ]
        )


pinTools :
    { s
        | hovered : HoverState.HoverState
        , pipelines : WebData (List Concourse.Pipeline)
        , userState : UserState
    }
    ->
        { b
            | pinnedVersion : Models.PinnedVersion
            , resourceIdentifier : Concourse.ResourceIdentifier
            , pinCommentLoading : Bool
            , isEditing : Bool
        }
    -> Html Message
pinTools session model =
    let
        pinBarVersion =
            Pinned.stable model.pinnedVersion
    in
    Html.div
        (id "pin-tools" :: Resource.Styles.pinTools (ME.isJust pinBarVersion))
        [ pinBar session model
        , commentBar session model
        ]


pinBar :
    { a
        | hovered : HoverState.HoverState
        , pipelines : WebData (List Concourse.Pipeline)
    }
    ->
        { b
            | pinnedVersion : Models.PinnedVersion
            , resourceIdentifier : Concourse.ResourceIdentifier
        }
    -> Html Message
pinBar session { pinnedVersion, resourceIdentifier } =
    let
        pinBarVersion =
            Pinned.stable pinnedVersion

        attrList : List ( Html.Attribute Message, Bool ) -> List (Html.Attribute Message)
        attrList =
            List.filter Tuple.second >> List.map Tuple.first

        isPinnedStatically =
            case pinnedVersion of
                PinnedStaticallyTo _ ->
                    True

                _ ->
                    False

        isPinnedDynamically =
            case pinnedVersion of
                PinnedDynamicallyTo _ _ ->
                    True

                _ ->
                    False

        archived =
            isPipelineArchived session resourceIdentifier
    in
    Html.div
        (attrList
            [ ( id <| toHtmlID PinBar, True )
            , ( onMouseEnter <| Hover <| Just PinBar, isPinnedStatically )
            , ( onMouseLeave <| Hover Nothing, isPinnedStatically )
            ]
            ++ Resource.Styles.pinBar (ME.isJust pinBarVersion)
        )
        (Icon.icon
            { sizePx = 14
            , image =
                if ME.isJust pinBarVersion then
                    Assets.PinIconWhite

                else
                    Assets.PinIconGrey
            }
            (attrList
                [ ( id <| toHtmlID PinIcon, True )
                , ( onClick <| Click PinIcon
                  , isPinnedDynamically && not archived
                  )
                , ( onMouseEnter <| Hover <| Just PinIcon
                  , isPinnedDynamically && not archived
                  )
                , ( onMouseLeave <| Hover Nothing
                  , isPinnedDynamically && not archived
                  )
                ]
                ++ Resource.Styles.pinIcon
                    { clickable = isPinnedDynamically && not archived
                    , hover = HoverState.isHovered PinIcon session.hovered
                    }
            )
            :: (case pinBarVersion of
                    Just v ->
                        [ viewVersion Resource.Styles.pinBarViewVersion v ]

                    _ ->
                        []
               )
        )


isPipelineArchived :
    { r | pipelines : WebData (List Concourse.Pipeline) }
    -> Concourse.ResourceIdentifier
    -> Bool
isPipelineArchived session id =
    lookupPipeline (byPipelineId id) session
        |> Maybe.map .archived
        |> Maybe.withDefault False


viewVersionedResources :
    { a
        | pipelines : WebData (List Concourse.Pipeline)
        , featureFlags : Concourse.FeatureFlags
    }
    ->
        { b
            | versions : Paginated Models.Version
            , pinnedVersion : Models.PinnedVersion
            , resourceIdentifier : Concourse.ResourceIdentifier
        }
    -> Html Message
viewVersionedResources session model =
    let
        archived =
            isPipelineArchived session model.resourceIdentifier
    in
    model
        |> versions
        |> List.map (\v -> viewVersionedResource { version = v, archived = archived, causalityEnabled = session.featureFlags.resource_causality })
        |> Html.ul [ class "list list-collapsable list-enableDisable resource-versions" ]


viewVersionedResource :
    { version : VersionPresenter
    , archived : Bool
    , causalityEnabled : Bool
    }
    -> Html Message
viewVersionedResource { version, archived, causalityEnabled } =
    Html.li
        (case ( version.pinState, version.enabled ) of
            ( Disabled, _ ) ->
                [ style "opacity" "0.5" ]

            ( NotThePinnedVersion, _ ) ->
                [ style "opacity" "0.5" ]

            ( _, Models.Disabled ) ->
                [ style "opacity" "0.5" ]

            _ ->
                []
        )
        (Html.div
            [ style "display" "flex"
            , style "margin" "5px 0px"
            ]
            ((if archived then
                []

              else
                [ viewEnabledCheckbox
                    { enabled = version.enabled
                    , id = version.id
                    , pinState = version.pinState
                    }
                , viewPinButton
                    { versionID = version.id
                    , pinState = version.pinState
                    }
                ]
             )
                ++ [ viewVersionHeader
                        { id = version.id
                        , version = version.version
                        , pinnedState = version.pinState
                        }
                   ]
            )
            :: (if version.expanded then
                    [ viewVersionBody
                        { inputTo = version.inputTo
                        , outputOf = version.outputOf
                        , versionId = version.id
                        , metadata = version.metadata
                        , causalityEnabled = causalityEnabled
                        }
                    ]

                else
                    []
               )
        )


viewVersionBody :
    { a
        | inputTo : List Concourse.Build
        , outputOf : List Concourse.Build
        , versionId : Concourse.VersionedResourceIdentifier
        , metadata : Concourse.Metadata
        , causalityEnabled : Bool
    }
    -> Html Message
viewVersionBody { inputTo, outputOf, versionId, metadata, causalityEnabled } =
    Html.div
        [ style "display" "flex"
        , style "padding" "5px 10px"
        ]
        [ viewInputsOrOutputs causalityEnabled Concourse.Downstream versionId inputTo
        , viewInputsOrOutputs causalityEnabled Concourse.Upstream versionId outputOf
        , Html.div [ class "vri metadata-container" ]
            [ Html.div [ class "list-collapsable-title" ] [ Html.text "metadata" ]
            , viewMetadata metadata
            ]
        ]


viewEnabledCheckbox :
    { a
        | enabled : Models.VersionEnabledState
        , id : Models.VersionId
        , pinState : VersionPinState
    }
    -> Html Message
viewEnabledCheckbox ({ enabled, id } as params) =
    let
        eventHandlers =
            [ onMouseOver <| Hover <| Just <| VersionToggle id
            , onMouseOut <| Hover Nothing
            ]
                ++ (case enabled of
                        Models.Enabled ->
                            [ onClick <| Click <| VersionToggle id ]

                        Models.Changing ->
                            []

                        Models.Disabled ->
                            [ onClick <| Click <| VersionToggle id ]
                   )
    in
    Html.div
        (Html.Attributes.id (toHtmlID <| VersionToggle id)
            :: Html.Attributes.attribute "aria-label" "Toggle Resource Version Enabled"
            :: Resource.Styles.enabledCheckbox params
            ++ eventHandlers
        )
        (case enabled of
            Models.Enabled ->
                []

            Models.Changing ->
                [ Spinner.spinner
                    { sizePx = 12.5
                    , margin = "6.25px"
                    }
                ]

            Models.Disabled ->
                []
        )


viewPinButton :
    { versionID : Models.VersionId
    , pinState : VersionPinState
    }
    -> Html Message
viewPinButton { versionID, pinState } =
    let
        eventHandlers =
            [ onMouseOver <| Hover <| Just <| PinButton versionID
            , onMouseOut <| Hover Nothing
            ]
                ++ (case pinState of
                        Enabled ->
                            [ onClick <| Click <| PinButton versionID ]

                        PinnedDynamically ->
                            [ onClick <| Click <| PinButton versionID ]

                        NotThePinnedVersion ->
                            [ onClick <| Click <| PinButton versionID ]

                        PinnedStatically ->
                            [ onMouseOver <| Hover <| Just <| PinButton versionID
                            , onMouseOut <| Hover Nothing
                            ]

                        Disabled ->
                            []

                        InTransition ->
                            []
                   )
    in
    Html.div
        (id (toHtmlID <| PinButton versionID)
            :: Html.Attributes.attribute "aria-label" "Pin Resource Version"
            :: Resource.Styles.pinButton pinState
            ++ eventHandlers
        )
        (case pinState of
            InTransition ->
                [ Spinner.spinner
                    { sizePx = 12.5
                    , margin = "6.25px"
                    }
                ]

            _ ->
                []
        )


viewVersionHeader :
    { a
        | id : Models.VersionId
        , version : Concourse.Version
        , pinnedState : VersionPinState
    }
    -> Html Message
viewVersionHeader { id, version, pinnedState } =
    Html.div
        ((onClick <| Click <| VersionHeader id)
            :: Resource.Styles.versionHeader pinnedState
        )
        [ viewVersion [] version ]


viewVersion : List (Html.Attribute Message) -> Concourse.Version -> Html Message
viewVersion attrs version =
    version
        |> Dict.map (always Html.text)
        |> DictView.view attrs


viewInputsOrOutputs : Bool -> Concourse.CausalityDirection -> Concourse.VersionedResourceIdentifier -> List Concourse.Build -> Html Message
viewInputsOrOutputs causalityEnabled direction versionId builds =
    Html.div [ class "vri" ] <|
        List.concat
            [ [ Html.div
                    [ style "line-height" "25px" ]
                    [ viewCausalityButton causalityEnabled direction versionId ]
              ]
            , viewBuilds <| listToMap builds
            ]


viewCausalityButton : Bool -> Concourse.CausalityDirection -> Concourse.VersionedResourceIdentifier -> Html Message
viewCausalityButton enabled dir versionId =
    let
        link =
            Routes.Causality
                { id = versionId
                , direction = dir
                , version = Nothing
                , groups = []
                }

        ( domID, text ) =
            case dir of
                Concourse.Downstream ->
                    ( InputsTo versionId, "inputs to" )

                Concourse.Upstream ->
                    ( OutputsOf versionId, "outputs of" )

        eventHandlers =
            [ onMouseOver <| Hover <| Just domID
            , onMouseOut <| Hover Nothing
            , onClick <| GoToRoute link
            ]
    in
    if enabled then
        Html.div
            [ style "line-height" "25px"
            , style "display" "flex"
            , style "justify-content" "space-between"
            ]
            [ Html.text text
            , Html.a
                ([ href (Routes.toString link)
                 , id (toHtmlID <| domID)
                 ]
                    ++ eventHandlers
                    ++ Resource.Styles.causalityButton
                )
                [ Html.text "view all" ]
            ]

    else
        Html.text text


viewMetadata : Concourse.Metadata -> Html Message
viewMetadata metadata =
    Html.dl [ class "build-metadata" ]
        (List.concatMap viewMetadataField metadata)


viewMetadataField : Concourse.MetadataField -> List (Html a)
viewMetadataField field =
    [ Html.dt [] [ Html.text field.name ]
    , Html.dd []
        [ Html.pre [ class "metadata-field" ] [ Html.text field.value ]
        ]
    ]


listToMap : List Concourse.Build -> Dict.Dict String (List Concourse.Build)
listToMap builds =
    let
        insertBuild =
            \build dict ->
                let
                    jobName =
                        case build.job of
                            Nothing ->
                                -- Jobless builds shouldn't appear on this page!
                                ""

                            Just job ->
                                job.jobName

                    oldList =
                        Dict.get jobName dict

                    newList =
                        case oldList of
                            Nothing ->
                                [ build ]

                            Just list ->
                                list ++ [ build ]
                in
                Dict.insert jobName newList dict
    in
    List.foldr insertBuild Dict.empty builds


viewBuilds : Dict.Dict String (List Concourse.Build) -> List (Html Message)
viewBuilds buildDict =
    List.concatMap (viewBuildsByJob buildDict) <| Dict.keys buildDict


formatDate : Time.Zone -> Time.Posix -> String
formatDate =
    DateFormat.format
        [ DateFormat.monthNameAbbreviated
        , DateFormat.text " "
        , DateFormat.dayOfMonthNumber
        , DateFormat.text " "
        , DateFormat.yearNumber
        , DateFormat.text " "
        , DateFormat.hourFixed
        , DateFormat.text ":"
        , DateFormat.minuteFixed
        , DateFormat.text ":"
        , DateFormat.secondFixed
        , DateFormat.text " "
        , DateFormat.amPmUppercase
        ]


viewLastChecked : Time.Zone -> Time.Posix -> Time.Posix -> Html a
viewLastChecked timeZone now date =
    let
        ago =
            Duration.between date now
    in
    Html.table [ id "last-checked" ]
        [ Html.tr
            []
            [ Html.td [] [ Html.text "checked" ]
            , Html.td
                [ title <| formatDate timeZone date ]
                [ Html.span [] [ Html.text (Duration.format ago ++ " ago") ] ]
            ]
        ]


viewBuildsByJob : Dict.Dict String (List Concourse.Build) -> String -> List (Html Message)
viewBuildsByJob buildDict jobName =
    let
        oneBuildToLi =
            \build ->
                case build.job of
                    Nothing ->
                        Html.li [ class <| Concourse.BuildStatus.show build.status ]
                            [ Html.text <| "#" ++ build.name ]

                    Just job ->
                        let
                            link =
                                Routes.Build
                                    { id =
                                        { teamName = job.teamName
                                        , pipelineName = job.pipelineName
                                        , pipelineInstanceVars = job.pipelineInstanceVars
                                        , jobName = job.jobName
                                        , buildName = build.name
                                        }
                                    , highlight = Routes.HighlightNothing
                                    , groups = []
                                    }
                        in
                        Html.li [ class <| Concourse.BuildStatus.show build.status ]
                            [ Html.a
                                [ StrictEvents.onLeftClick <| GoToRoute link
                                , href (Routes.toString link)
                                ]
                                [ Html.text <| "#" ++ build.name ]
                            ]
    in
    [ Html.h3 [ class "man pas ansi-bright-black-bg" ] [ Html.text jobName ]
    , Html.ul [ class "builds-list" ]
        (case Dict.get jobName buildDict of
            Nothing ->
                []

            -- never happens
            Just buildList ->
                List.map oneBuildToLi buildList
        )
    ]


fetchDataForExpandedVersions : Model -> List Effect
fetchDataForExpandedVersions model =
    model.versions.content
        |> List.filter .expanded
        |> List.concatMap (\v -> [ FetchInputTo v.id, FetchOutputOf v.id ])


updateOutput :
    (OutputModel -> ( OutputModel, List Effect ))
    -> ET Model
updateOutput updater ( model, effects ) =
    case model.output of
        Just output ->
            let
                ( newOutput, outputEffects ) =
                    updater output

                newModel =
                    { model
                        | output =
                            -- model.output must be equal-by-reference
                            -- to its previous value when passed
                            -- into `Html.Lazy.lazy3` below.
                            if newOutput /= output then
                                Just newOutput

                            else
                                model.output
                    }
            in
            ( newModel, effects ++ outputEffects )

        _ ->
            ( model, effects )


viewBuildOutput : Time.Zone -> HoverState.HoverState -> Maybe OutputModel -> Html Message
viewBuildOutput timeZone hovered output =
    case output of
        Just o ->
            Build.Output.Output.view
                { timeZone = timeZone, hovered = hovered }
                o

        Nothing ->
            Html.div [] []
