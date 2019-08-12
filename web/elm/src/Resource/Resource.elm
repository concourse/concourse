module Resource.Resource exposing
    ( Flags
    , changeToResource
    , documentTitle
    , getUpdateMessage
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , update
    , view
    , viewPinButton
    , viewVersionBody
    , viewVersionHeader
    )

import Application.Models exposing (Session)
import Concourse
import Concourse.BuildStatus
import Concourse.Pagination
    exposing
        ( Page
        , Paginated
        , chevron
        , chevronContainer
        , equal
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
        , placeholder
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
import Http
import Keyboard
import List.Extra
import Login.Login as Login
import Maybe.Extra as ME
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
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
import Resource.Models as Models exposing (Model)
import Resource.Styles
import Routes
import SideBar.SideBar as SideBar
import StrictEvents
import Svg
import Svg.Attributes as SvgAttributes
import Time
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
    }


init : Flags -> ( Model, List Effect )
init flags =
    let
        model =
            { resourceIdentifier = flags.resourceId
            , pageStatus = Err Models.Empty
            , checkStatus = Models.CheckingSuccessfully
            , checkError = ""
            , checkSetupError = ""
            , lastChecked = Nothing
            , pinnedVersion = NotPinned
            , currentPage = flags.paging
            , versions =
                { content = []
                , pagination = { previousPage = Nothing, nextPage = Nothing }
                }
            , now = Nothing
            , pinCommentLoading = False
            , textAreaFocused = False
            , isUserMenuExpanded = False
            , icon = Nothing
            }
    in
    ( model
    , [ FetchResource flags.resourceId
      , FetchVersionedResources flags.resourceId flags.paging
      , GetCurrentTimeZone
      , FetchPipelines
      ]
    )


changeToResource : Flags -> ET Model
changeToResource flags ( model, effects ) =
    ( { model
        | currentPage = flags.paging
        , versions =
            { content = []
            , pagination = { previousPage = Nothing, nextPage = Nothing }
            }
      }
    , effects ++ [ FetchVersionedResources model.resourceIdentifier flags.paging ]
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


subscriptions : List Subscription
subscriptions =
    [ OnClockTick Subscription.FiveSeconds
    , OnClockTick Subscription.OneSecond
    , OnKeyDown
    , OnKeyUp
    ]


handleCallback : Callback -> Session -> ET Model
handleCallback callback session ( model, effects ) =
    case callback of
        ResourceFetched (Ok resource) ->
            ( { model
                | pageStatus = Ok ()
                , resourceIdentifier =
                    { teamName = resource.teamName
                    , pipelineName = resource.pipelineName
                    , resourceName = resource.name
                    }
                , checkStatus =
                    if resource.failingToCheck then
                        Models.FailingToCheck

                    else
                        Models.CheckingSuccessfully
                , checkError = resource.checkError
                , checkSetupError = resource.checkSetupError
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
            )

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

        VersionedResourcesFetched (Ok ( requestedPage, paginated )) ->
            let
                fetchedPage =
                    permalink paginated.content

                versions =
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
                                                , resourceName = model.resourceIdentifier.resourceName
                                                , versionID = vr.id
                                                }
                                            , version = vr.version
                                            , metadata = vr.metadata
                                            , enabled = enabledStateAccordingToServer
                                            , expanded = False
                                            , inputTo = []
                                            , outputOf = []
                                            , showTooltip = False
                                            }
                                )
                    }

                newModel =
                    \newPage ->
                        { model
                            | versions = versions
                            , currentPage = newPage
                        }

                chosenModelWith =
                    \requestedPageUnwrapped ->
                        case model.currentPage of
                            Nothing ->
                                newModel <| Just fetchedPage

                            Just page ->
                                if Concourse.Pagination.equal page requestedPageUnwrapped then
                                    newModel <| requestedPage

                                else
                                    model
            in
            case requestedPage of
                Nothing ->
                    ( newModel (Just fetchedPage), effects )

                Just requestedPageUnwrapped ->
                    ( chosenModelWith requestedPageUnwrapped
                    , effects
                    )

        InputToFetched (Ok ( versionID, builds )) ->
            ( updateVersion versionID (\v -> { v | inputTo = builds }) model
            , effects
            )

        OutputOfFetched (Ok ( versionID, builds )) ->
            ( updateVersion versionID (\v -> { v | outputOf = builds }) model
            , effects
            )

        VersionPinned (Ok ()) ->
            case ( session.userState, model.now, model.pinnedVersion ) of
                ( UserStateLoggedIn user, Just time, PinningTo pinningTo ) ->
                    let
                        commentText =
                            "pinned by "
                                ++ Login.userDisplayName user
                                ++ " at "
                                ++ formatDate session.timeZone time
                    in
                    ( { model
                        | pinnedVersion =
                            model.versions.content
                                |> List.Extra.find (\v -> v.id == pinningTo)
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

        Checked (Ok ()) ->
            ( { model | checkStatus = Models.CheckingSuccessfully }
            , effects
                ++ [ FetchResource model.resourceIdentifier
                   , FetchVersionedResources
                        model.resourceIdentifier
                        model.currentPage
                   ]
            )

        Checked (Err err) ->
            ( { model | checkStatus = Models.FailingToCheck }
            , case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        effects ++ [ RedirectToLogin ]

                    else
                        effects ++ [ FetchResource model.resourceIdentifier ]

                _ ->
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
              }
            , effects ++ [ FetchResource model.resourceIdentifier ]
            )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
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
            ( { model | now = Just time }, effects )

        ClockTicked FiveSeconds _ ->
            ( model
            , effects
                ++ [ FetchResource model.resourceIdentifier
                   , FetchVersionedResources model.resourceIdentifier model.currentPage
                   , FetchPipelines
                   ]
                ++ fetchDataForExpandedVersions model
            )

        _ ->
            ( model, effects )


update : Message -> ET Model
update msg ( model, effects ) =
    case msg of
        Click (PaginationButton page) ->
            ( { model
                | currentPage = Just page
              }
            , effects
                ++ [ FetchVersionedResources model.resourceIdentifier <| Just page
                   , NavigateTo <|
                        Routes.toString <|
                            Routes.Resource
                                { id = model.resourceIdentifier
                                , page = Just page
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
                PinnedDynamicallyTo _ _ ->
                    ( { model
                        | pinnedVersion =
                            Pinned.startUnpinning model.pinnedVersion
                      }
                    , effects ++ [ DoUnpinVersion model.resourceIdentifier ]
                    )

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
                ( { model | checkStatus = Models.CurrentlyChecking }
                , effects ++ [ DoCheck model.resourceIdentifier ]
                )

            else
                ( model, effects ++ [ RedirectToLogin ] )

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
            ( { model | pinnedVersion = newPinnedVersion }, effects )

        Click SaveCommentButton ->
            case model.pinnedVersion of
                PinnedDynamicallyTo commentState _ ->
                    ( { model | pinCommentLoading = True }
                    , effects
                        ++ [ SetPinComment
                                model.resourceIdentifier
                                commentState.comment
                           ]
                    )

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

        versions : Paginated Models.Version
        versions =
            model.versions
    in
    { model | versions = { versions | content = newVersionsContent } }


permalink : List Concourse.VersionedResource -> Page
permalink versionedResources =
    case List.head versionedResources of
        Nothing ->
            { direction = Concourse.Pagination.Since 0
            , limit = 100
            }

        Just version ->
            { direction = Concourse.Pagination.From version.id
            , limit = List.length versionedResources
            }


documentTitle : Model -> String
documentTitle model =
    model.resourceIdentifier.resourceName


view : Session -> Model -> Html Message
view session model =
    let
        route =
            Routes.Resource
                { id = model.resourceIdentifier
                , page = Nothing
                }
    in
    Html.div
        (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
        [ Html.div
            (id "top-bar-app" :: Views.Styles.topBar False)
            [ SideBar.hamburgerMenu session
            , TopBar.concourseLogo
            , TopBar.breadcrumbs route
            , Login.view session.userState model False
            ]
        , Html.div
            (id "page-below-top-bar" :: Views.Styles.pageBelowTopBar route)
            [ SideBar.view session
                (Just
                    { pipelineName = model.resourceIdentifier.pipelineName
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
                    , commentBar session model
                    ]
            ]
        ]


header : Session -> Model -> Html Message
header session model =
    let
        lastCheckedView =
            case ( model.now, model.lastChecked ) of
                ( Just now, Just date ) ->
                    viewLastChecked session.timeZone now date

                ( _, _ ) ->
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
        , pinBar session model
        , paginationMenu session model
        ]


body :
    { a | userState : UserState, hovered : HoverState.HoverState }
    -> Model
    -> Html Message
body session model =
    let
        sectionModel =
            { checkStatus = model.checkStatus
            , checkSetupError = model.checkSetupError
            , checkError = model.checkError
            , hovered = session.hovered
            , userState = session.userState
            , teamName = model.resourceIdentifier.teamName
            }
    in
    Html.div
        (id "body" :: Resource.Styles.body)
        [ checkSection sectionModel
        , viewVersionedResources session model
        ]


paginationMenu :
    { a | hovered : HoverState.HoverState }
    ->
        { b
            | versions : Paginated Models.Version
            , resourceIdentifier : Concourse.ResourceIdentifier
        }
    -> Html Message
paginationMenu { hovered } { versions, resourceIdentifier } =
    let
        previousButtonEventHandler =
            case versions.pagination.previousPage of
                Nothing ->
                    []

                Just pp ->
                    [ onClick <| Click <| PaginationButton pp ]

        nextButtonEventHandler =
            case versions.pagination.nextPage of
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
        [ case versions.pagination.previousPage of
            Nothing ->
                Html.div
                    chevronContainer
                    [ Html.div
                        (chevron
                            { direction = "left"
                            , enabled = False
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
                                Routes.Resource { id = resourceIdentifier, page = Just page }
                         , attribute "aria-label" "Previous Page"
                         ]
                            ++ chevron
                                { direction = "left"
                                , enabled = True
                                , hovered = HoverState.isHovered PreviousPageButton hovered
                                }
                        )
                        []
                    ]
        , case versions.pagination.nextPage of
            Nothing ->
                Html.div
                    chevronContainer
                    [ Html.div
                        (chevron
                            { direction = "right"
                            , enabled = False
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
                                Routes.Resource { id = resourceIdentifier, page = Just page }
                         , attribute "aria-label" "Next Page"
                         ]
                            ++ chevron
                                { direction = "right"
                                , enabled = True
                                , hovered = HoverState.isHovered NextPageButton hovered
                                }
                        )
                        []
                    ]
        ]


checkSection :
    { a
        | checkStatus : Models.CheckStatus
        , checkSetupError : String
        , checkError : String
        , hovered : HoverState.HoverState
        , userState : UserState
        , teamName : String
    }
    -> Html Message
checkSection ({ checkStatus, checkSetupError, checkError } as model) =
    let
        failingToCheck =
            checkStatus == Models.FailingToCheck

        checkMessage =
            case checkStatus of
                Models.FailingToCheck ->
                    "checking failed"

                Models.CurrentlyChecking ->
                    "currently checking"

                Models.CheckingSuccessfully ->
                    "checking successfully"

        stepBody =
            if failingToCheck then
                if not (String.isEmpty checkSetupError) then
                    [ Html.div [ class "step-body" ]
                        [ Html.pre [] [ Html.text checkSetupError ]
                        ]
                    ]

                else
                    [ Html.div [ class "step-body" ]
                        [ Html.pre [] [ Html.text checkError ]
                        ]
                    ]

            else
                []

        statusIcon =
            case checkStatus of
                Models.CurrentlyChecking ->
                    Spinner.spinner
                        { sizePx = 14
                        , margin = "7px"
                        }

                _ ->
                    Icon.icon
                        { sizePx = 28
                        , image =
                            if failingToCheck then
                                "ic-exclamation-triangle.svg"

                            else
                                "ic-success-check.svg"
                        }
                        Resource.Styles.checkStatusIcon

        statusBar =
            Html.div
                Resource.Styles.checkBarStatus
                [ Html.h3 [] [ Html.text checkMessage ]
                , statusIcon
                ]

        checkBar =
            Html.div
                [ style "display" "flex" ]
                [ checkButton model
                , statusBar
                ]
    in
    Html.div [ class "resource-check-status" ] <| checkBar :: stepBody


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
            checkStatus == Models.CurrentlyChecking

        isAnonymous =
            UserState.isAnonymous userState

        isClickable =
            (isAnonymous || isMember)
                && not isCurrentlyChecking

        isHighlighted =
            (isClickable && isHovered) || isCurrentlyChecking
    in
    Html.div
        ([ onMouseEnter <| Hover <| Just <| CheckButton isMember
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
            , image = "baseline-refresh-24px.svg"
            }
            (Resource.Styles.checkButtonIcon isHighlighted)
        ]


commentBar :
    { a | userState : UserState, hovered : HoverState.HoverState }
    ->
        { b
            | pinnedVersion : Models.PinnedVersion
            , resourceIdentifier : Concourse.ResourceIdentifier
            , pinCommentLoading : Bool
        }
    -> Html Message
commentBar { userState, hovered } { resourceIdentifier, pinnedVersion, pinCommentLoading } =
    case pinnedVersion of
        PinnedDynamicallyTo commentState v ->
            let
                version =
                    viewVersion
                        [ Html.Attributes.style "align-self" "center" ]
                        v
            in
            Html.div
                (id "comment-bar" :: Resource.Styles.commentBar)
                [ Html.div Resource.Styles.commentBarContent <|
                    let
                        commentBarHeader =
                            Html.div
                                Resource.Styles.commentBarHeader
                                [ Html.div
                                    Resource.Styles.commentBarIconContainer
                                    [ Icon.icon
                                        { sizePx = 24
                                        , image = "baseline-message.svg"
                                        }
                                        Resource.Styles.commentBarMessageIcon
                                    , Icon.icon
                                        { sizePx = 20
                                        , image = "pin-ic-white.svg"
                                        }
                                        Resource.Styles.commentBarPinIcon
                                    ]
                                , version
                                ]
                    in
                    if
                        UserState.isMember
                            { teamName = resourceIdentifier.teamName
                            , userState = userState
                            }
                    then
                        [ commentBarHeader
                        , Html.textarea
                            ([ onInput EditComment
                             , value commentState.comment
                             , placeholder "enter a comment"
                             , onFocus FocusTextArea
                             , onBlur BlurTextArea
                             ]
                                ++ Resource.Styles.commentTextArea
                            )
                            []
                        , Html.button
                            (let
                                commentChanged =
                                    commentState.comment
                                        /= commentState.pristineComment
                             in
                             [ onMouseEnter <| Hover <| Just SaveCommentButton
                             , onMouseLeave <| Hover Nothing
                             , onClick <| Click SaveCommentButton
                             ]
                                ++ Resource.Styles.commentSaveButton
                                    { isHovered =
                                        not pinCommentLoading
                                            && commentChanged
                                            && HoverState.isHovered SaveCommentButton hovered
                                    , commentChanged = commentChanged
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
                        ]

                    else
                        [ commentBarHeader
                        , Html.pre
                            Resource.Styles.commentText
                            [ Html.text commentState.pristineComment ]
                        , Html.div [ style "height" "24px" ] []
                        ]
                ]

        _ ->
            Html.text ""


pinBar :
    { a | hovered : HoverState.HoverState }
    -> { b | pinnedVersion : Models.PinnedVersion }
    -> Html Message
pinBar { hovered } { pinnedVersion } =
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
    in
    Html.div
        (attrList
            [ ( id "pin-bar", True )
            , ( onMouseEnter <| Hover <| Just PinBar, isPinnedStatically )
            , ( onMouseLeave <| Hover Nothing, isPinnedStatically )
            ]
            ++ Resource.Styles.pinBar (ME.isJust pinBarVersion)
        )
        (Icon.icon
            { sizePx = 25
            , image =
                if ME.isJust pinBarVersion then
                    "pin-ic-white.svg"

                else
                    "pin-ic-grey.svg"
            }
            (attrList
                [ ( id "pin-icon", True )
                , ( onClick <| Click PinIcon
                  , isPinnedDynamically
                  )
                , ( onMouseEnter <| Hover <| Just PinIcon
                  , isPinnedDynamically
                  )
                , ( onMouseLeave <| Hover Nothing, True )
                ]
                ++ Resource.Styles.pinIcon
                    { isPinnedDynamically = isPinnedDynamically
                    , hover = HoverState.isHovered PinIcon hovered
                    }
            )
            :: (case pinBarVersion of
                    Just v ->
                        [ viewVersion [] v ]

                    _ ->
                        []
               )
            ++ (if HoverState.isHovered PinBar hovered then
                    [ Html.div
                        (id "pin-bar-tooltip" :: Resource.Styles.pinBarTooltip)
                        [ Html.text "pinned in pipeline config" ]
                    ]

                else
                    []
               )
        )


viewVersionedResources :
    { a | hovered : HoverState.HoverState }
    ->
        { b
            | versions : Paginated Models.Version
            , pinnedVersion : Models.PinnedVersion
        }
    -> Html Message
viewVersionedResources { hovered } { versions, pinnedVersion } =
    versions.content
        |> List.map
            (\v ->
                viewVersionedResource
                    { version = v
                    , pinnedVersion = pinnedVersion
                    , hovered = hovered
                    }
            )
        |> Html.ul [ class "list list-collapsable list-enableDisable resource-versions" ]


viewVersionedResource :
    { version : Models.Version
    , pinnedVersion : Models.PinnedVersion
    , hovered : HoverState.HoverState
    }
    -> Html Message
viewVersionedResource { version, pinnedVersion, hovered } =
    let
        pinState =
            case Pinned.pinState version.version version.id pinnedVersion of
                PinnedStatically _ ->
                    PinnedStatically version.showTooltip

                x ->
                    x
    in
    Html.li
        (case ( pinState, version.enabled ) of
            ( Disabled, _ ) ->
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
            [ viewEnabledCheckbox
                { enabled = version.enabled
                , id = version.id
                , pinState = pinState
                }
            , viewPinButton
                { versionID = version.id
                , pinState = pinState
                , hovered = hovered
                }
            , viewVersionHeader
                { id = version.id
                , version = version.version
                , pinnedState = pinState
                }
            ]
            :: (if version.expanded then
                    [ viewVersionBody
                        { inputTo = version.inputTo
                        , outputOf = version.outputOf
                        , metadata = version.metadata
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
        , metadata : Concourse.Metadata
    }
    -> Html Message
viewVersionBody { inputTo, outputOf, metadata } =
    Html.div
        [ style "display" "flex"
        , style "padding" "5px 10px"
        ]
        [ Html.div [ class "vri" ] <|
            List.concat
                [ [ Html.div [ style "line-height" "25px" ] [ Html.text "inputs to" ] ]
                , viewBuilds <| listToMap inputTo
                ]
        , Html.div [ class "vri" ] <|
            List.concat
                [ [ Html.div [ style "line-height" "25px" ] [ Html.text "outputs of" ] ]
                , viewBuilds <| listToMap outputOf
                ]
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
        clickHandler =
            case enabled of
                Models.Enabled ->
                    [ onClick <| Click <| VersionToggle id ]

                Models.Changing ->
                    []

                Models.Disabled ->
                    [ onClick <| Click <| VersionToggle id ]
    in
    Html.div
        (Html.Attributes.attribute "aria-label" "Toggle Resource Version Enabled"
            :: Resource.Styles.enabledCheckbox params
            ++ clickHandler
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
    , hovered : HoverState.HoverState
    }
    -> Html Message
viewPinButton { versionID, pinState, hovered } =
    let
        eventHandlers =
            case pinState of
                Enabled ->
                    [ onClick <| Click <| PinButton versionID ]

                PinnedDynamically ->
                    [ onClick <| Click <| PinButton versionID ]

                PinnedStatically _ ->
                    [ onMouseOver <| Hover <| Just <| PinButton versionID
                    , onMouseOut <| Hover Nothing
                    ]

                Disabled ->
                    []

                InTransition ->
                    []
    in
    Html.div
        (Html.Attributes.attribute "aria-label" "Pin Resource Version"
            :: Resource.Styles.pinButton pinState
            ++ eventHandlers
        )
        (case pinState of
            PinnedStatically _ ->
                if HoverState.isHovered (PinButton versionID) hovered then
                    [ Html.div
                        Resource.Styles.pinButtonTooltip
                        [ Html.text "enable via pipeline config" ]
                    ]

                else
                    []

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
                                        , jobName = job.jobName
                                        , buildName = build.name
                                        }
                                    , highlight = Routes.HighlightNothing
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
