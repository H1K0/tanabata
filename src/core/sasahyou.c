#include <malloc.h>
#include <string.h>
#include <time.h>
#include <stdio.h>

#include "../../include/core.h"

int sasahyou_init(Sasahyou *sasahyou) {
    sasahyou->created_ts = time(NULL);
    sasahyou->modified_ts = sasahyou->created_ts;
    sasahyou->size = 0;
    sasahyou->removed_cnt = 0;
    sasahyou->contents = NULL;
    sasahyou->file = NULL;
    return 0;
}

int sasahyou_free(Sasahyou *sasahyou) {
    for (uint64_t i = 0; i < sasahyou->size; i++) {
        free(sasahyou->contents[i].path);
    }
    free(sasahyou->contents);
    if (sasahyou->file != NULL) {
        fclose(sasahyou->file);
    }
    return 0;
}

int sasahyou_load(Sasahyou *sasahyou) {
    if (sasahyou->file == NULL) {
        fprintf(stderr, "Failed to load sasahyou: file not specified\n");
        return 1;
    }
    uint16_t signature[4];
    rewind(sasahyou->file);
    fread(signature, 2, 4, sasahyou->file);
    if (memcmp(signature, SASAHYOU_SIG, 8) != 0) {
        fprintf(stderr, "Failed to load sasahyou: invalid signature\n");
        return 1;
    }
    fread(&sasahyou->created_ts, 8, 1, sasahyou->file);
    fread(&sasahyou->modified_ts, 8, 1, sasahyou->file);
    fread(&sasahyou->size, 8, 1, sasahyou->file);
    sasahyou->removed_cnt = 0;
    sasahyou->contents = malloc(sasahyou->size * sizeof(Sasa));
    size_t max_path_len = SIZE_MAX;
    for (uint64_t i = 0; i < sasahyou->size; i++) {
        if (fgetc(sasahyou->file) != 0) {
            sasahyou->contents[i].id = i + 1;
            fread(&sasahyou->contents[i].created_ts, 8, 1, sasahyou->file);
            getdelim(&sasahyou->contents[i].path, &max_path_len, 0, sasahyou->file);
        } else {
            sasahyou->contents[i].id = 0;
            sasahyou->removed_cnt++;
        }
    }
    return 0;
}

int sasahyou_save(Sasahyou *sasahyou) {
    if (sasahyou->file == NULL) {
        fprintf(stderr, "Failed to save sasahyou: file not specified\n");
        return 1;
    }
    rewind(sasahyou->file);
    fwrite(SASAHYOU_SIG, 2, 4, sasahyou->file);
    fwrite(&sasahyou->created_ts, 8, 1, sasahyou->file);
    fwrite(&sasahyou->modified_ts, 8, 1, sasahyou->file);
    fwrite(&sasahyou->size, 8, 1, sasahyou->file);
    fflush(sasahyou->file);
    for (uint64_t i = 0; i < sasahyou->size; i++) {
        if (sasahyou->contents[i].id != 0) {
            fputc(-1, sasahyou->file);
            fwrite(&sasahyou->contents[i].created_ts, 8, 1, sasahyou->file);
            fputs(sasahyou->contents[i].path, sasahyou->file);
            fputc(0, sasahyou->file);
        } else {
            fputc(0, sasahyou->file);
        }
    }
    fflush(sasahyou->file);
    return 0;
}

int sasahyou_open(Sasahyou *sasahyou, const char *path) {
    sasahyou->file = fopen(path, "r+b");
    if (sasahyou->file == NULL) {
        fprintf(stderr, "Failed to dump sasahyou: failed to open file '%s'\n", path);
        return 1;
    }
    return sasahyou_load(sasahyou);
}

int sasahyou_dump(Sasahyou *sasahyou, const char *path) {
    sasahyou->file = fopen(path, "w+b");
    if (sasahyou->file == NULL) {
        fprintf(stderr, "Failed to dump sasahyou: failed to open file '%s'\n", path);
        return 1;
    }
    return sasahyou_save(sasahyou);
}

int sasa_add(Sasahyou *sasahyou, const char *path) {
    if (sasahyou->size == -1) {
        fprintf(stderr, "Failed to add sasa: sasahyou is full\n");
        return 1;
    }
    Sasa newbie;
    newbie.created_ts = (uint64_t) time(NULL);
    size_t path_size = strlen(path);
    newbie.path = malloc(path_size + 1);
    strcpy(newbie.path, path);
    newbie.path[path_size] = 0;
    sasahyou->size++;
    newbie.id = sasahyou->size;
    sasahyou->contents = realloc(sasahyou->contents, sasahyou->size * sizeof(Sasa));
    sasahyou->contents[sasahyou->size - 1] = newbie;
    sasahyou->modified_ts = newbie.created_ts;
    return 0;
}

int sasa_rem_by_id(Sasahyou *sasahyou, uint64_t sasa_id) {
    if (sasa_id == 0) {
        fprintf(stderr, "Failed to remove sasa: got zero ID\n");
        return 1;
    }
    for (uint64_t i = 0; i < sasahyou->size; i++) {
        if (sasahyou->contents[i].id == sasa_id) {
            sasahyou->modified_ts = time(NULL);
            sasahyou->contents[i].id = 0;
            sasahyou->removed_cnt++;
            return 0;
        }
    }
    fprintf(stderr, "Failed to remove sasa: target sasa does not exist\n");
    return 1;
}

int sasa_rem_by_path(Sasahyou *sasahyou, const char *path) {
    for (uint64_t i = 0; i < sasahyou->size; i++) {
        if (strcmp(sasahyou->contents[i].path, path) == 0) {
            if (sasahyou->contents[i].id != 0) {
                sasahyou->modified_ts = time(NULL);
                sasahyou->contents[i].id = 0;
                sasahyou->removed_cnt++;
                return 0;
            } else {
                fprintf(stderr, "Failed to remove sasa: target sasa is already removed\n");
                return 1;
            }
        }
    }
    fprintf(stderr, "Failed to remove sasa: target sasa does not exist\n");
    return 1;
}
